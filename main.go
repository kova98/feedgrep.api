package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/jmoiron/sqlx"
	_ "github.com/joho/godotenv/autoload"
	"github.com/kova98/feedgrep.api/monitor"
	"github.com/kova98/feedgrep.api/notifiers"
	"github.com/kova98/feedgrep.api/sources"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/handlers"
)

var (
	auth           *handlers.AuthHandler
	apiMonitor     *monitor.APIMonitor
	UserContextKey = "user"
)

//go:embed data/migrations/*.sql
var embedMigrations embed.FS

func main() {
	config.LoadConfig()

	opts := slog.HandlerOptions{Level: config.Config.LogLevel}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &opts))
	slog.SetDefault(logger)

	db, err := sqlx.Connect("postgres", config.Config.PostgresURL)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(90)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := data.RunMigrations(db.DB, embedMigrations); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	usersRepo := repos.NewUserRepo(db)
	users := handlers.NewUserHandler(usersRepo)
	keywordRepo := repos.NewKeywordRepo(db)
	matchRepo := repos.NewMatchRepo(db)
	rateLimitRepo := repos.NewRateLimitRepo(db)
	authActionTokenRepo := repos.NewAuthActionTokenRepo(db)

	// TODO: clean this shit up
	smartFilterGenerator := handlers.NewSmartFilterGenerator(config.Config.OpenAIAPIKey, config.Config.OpenAIModel)

	keywords := handlers.NewKeywordHandler(keywordRepo, matchRepo, rateLimitRepo, config.Config.SearchAPIURL, smartFilterGenerator)
	matches := handlers.NewMatchHandler(matchRepo)

	arcticShiftMonitor := monitor.NewArcticShiftMonitor()
	arcticShiftMonitor.Register(prometheus.DefaultRegisterer)
	apiMonitor = monitor.NewAPIMonitor()
	apiMonitor.Register(prometheus.DefaultRegisterer)
	notificationsMonitor := monitor.NewNotificationsMonitor()
	notificationsMonitor.Register(prometheus.DefaultRegisterer)
	keywordMonitor := monitor.NewKeywordMonitor()
	keywordMonitor.Register(prometheus.DefaultRegisterer)

	arcticShiftPoller := sources.NewArcticShiftPoller(logger, keywordRepo, matchRepo, arcticShiftMonitor, keywordMonitor)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if config.Config.EnableArcticShift {
		go arcticShiftPoller.StartPolling(ctx)
	}

	mailer := notifiers.NewMailer(
		config.Config.SMTPHost,
		config.Config.SMTPPort,
		config.Config.SMTPFrom,
		config.Config.SMTPUsername,
		config.Config.SMTPPassword,
		config.Config.AppBaseURL,
	)
	keycloakClient := gocloak.NewClient(config.Config.KeycloakURL)
	auth = handlers.NewAuthHandler(keycloakClient, authActionTokenRepo, mailer)
	go auth.StartTokenTicker()

	notifier := NewNotifier(mailer, matchRepo, usersRepo, notificationsMonitor)
	go notifier.Start(ctx)

	feedback := handlers.NewFeedbackHandler(mailer)

	mux := http.NewServeMux()

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.Handle("POST /auth/login", public(auth.Login))
	mux.Handle("POST /auth/register", public(auth.Register))
	mux.Handle("POST /auth/reset-password", public(auth.ResetPassword))
	mux.Handle("POST /auth/reset-password/confirm", public(auth.ConfirmResetPassword))
	mux.Handle("POST /auth/refresh", public(auth.Refresh))
	mux.Handle("POST /auth/logout", public(auth.Logout))

	mux.Handle("POST /users/init", private(users.InitializeUser))
	mux.Handle("POST /keywords", private(keywords.CreateKeyword))
	mux.Handle("GET /keywords", private(keywords.GetKeywords))
	mux.Handle("POST /keywords/generate-smart-filter", private(keywords.GenerateSmartFilter))
	mux.Handle("GET /keywords/{id}", private(keywords.GetKeyword))
	mux.Handle("PUT /keywords/{id}", private(keywords.UpdateKeyword))
	mux.Handle("DELETE /keywords/{id}", private(keywords.DeleteKeyword))
	mux.Handle("GET /keywords/{id}/matches", private(keywords.GetKeywordMatches))
	mux.Handle("GET /keywords/{id}/match-activity", private(keywords.GetKeywordMatchActivity))
	mux.Handle("GET /keywords/{id}/matched-subreddits", private(keywords.GetKeywordMatchedSubreddits))
	mux.Handle("GET /keywords/{id}/historical-stream", privateHTTP(keywords.StreamHistoricalSmartMatches))
	mux.Handle("GET /matches", private(matches.GetMatches))
	mux.Handle("PUT /matches/{id}/seen", private(matches.UpdateMatchSeen))
	mux.Handle("POST /feedback", private(feedback.SubmitFeedback))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigCh
		slog.Info("Shutting down...")
		cancel()
		if err := db.Close(); err != nil {
			slog.Error("failed to close database connection", "error", err)
		}
		os.Exit(0)

	}()

	slog.Info("Starting server on port 8080")
	err = http.ListenAndServe(":8080", withCORS(mux))
	if err != nil {
		slog.Error("failed to start server", "error", err)
	}
}

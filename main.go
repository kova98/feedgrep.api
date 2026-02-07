package main

import (
	"context"
	"embed"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/jmoiron/sqlx"
	_ "github.com/joho/godotenv/autoload"
	"github.com/kova98/feedgrep.api/notifiers"
	"github.com/kova98/feedgrep.api/sources"
	_ "github.com/lib/pq"
	"golang.org/x/net/proxy"

	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/handlers"
)

var (
	auth           *handlers.AuthHandler
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

	keywords := handlers.NewKeywordHandler(keywordRepo)
	matches := handlers.NewMatchHandler(matchRepo)
	keycloakClient := gocloak.NewClient(config.Config.KeycloakURL)
	auth = handlers.NewAuthHandler(keycloakClient)
	go auth.StartTokenTicker()

	client, err := httpClient(config.Config.ProxyURL)
	if err != nil {
		slog.Error("failed to create http client", "error", err)
		os.Exit(1)
	}
	pollHandler := sources.NewRedditPoller(logger, client, keywordRepo, matchRepo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if config.Config.EnableRedditPolling {
		go pollHandler.StartPolling(ctx)
	}

	mailer := notifiers.NewMailer(
		config.Config.SMTPHost,
		config.Config.SMTPPort,
		config.Config.SMTPFrom,
		config.Config.SMTPPassword,
	)
	notifier := NewNotifier(mailer, matchRepo, usersRepo)
	go notifier.Start(ctx)

	feedback := handlers.NewFeedbackHandler(mailer)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /users/init", private(users.InitializeUser))

	mux.HandleFunc("POST /keywords", private(keywords.CreateKeyword))
	mux.HandleFunc("GET /keywords", private(keywords.GetKeywords))
	mux.HandleFunc("GET /keywords/{id}", private(keywords.GetKeyword))
	mux.HandleFunc("PUT /keywords/{id}", private(keywords.UpdateKeyword))
	mux.HandleFunc("DELETE /keywords/{id}", private(keywords.DeleteKeyword))

	mux.HandleFunc("GET /matches", private(matches.GetMatches))

	mux.HandleFunc("POST /feedback", private(feedback.SubmitFeedback))

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

func httpClient(proxyURL string) (*http.Client, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	if proxyURL == "" {
		return client, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "socks5" {
		return client, nil
	}

	// SOCKS5 proxy with authentication
	var auth *proxy.Auth
	if parsedURL.User != nil {
		password, _ := parsedURL.User.Password()
		auth = &proxy.Auth{
			User:     parsedURL.User.Username(),
			Password: password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	slog.Info("using SOCKS5 proxy", "proxy", parsedURL.Host)

	return client, nil
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func private(handler handlers.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyHeader := r.Header.Get("x-api-key")
		authHeader := r.Header.Get("Authorization")
		result := auth.GetUser(r.Context(), keyHeader, authHeader)
		if result.Code != http.StatusOK {
			slog.Debug("unauthorized request", "path", r.URL.Path)
			writeResult(w, result)
			return
		}

		user := result.Body.(data.User)
		ctx := context.WithValue(r.Context(), UserContextKey, user)

		public(handler)(w, r.WithContext(ctx))
	}
}

func public(handler handlers.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ts := time.Now()
		res := handler(w, r)
		elapsedMs := time.Since(ts).Milliseconds()
		slog.Debug("req", "method", r.Method, "path", r.URL.Path, "code", res.Code, "elapsed", elapsedMs)
		writeResult(w, res)
	}
}

func writeResult(w http.ResponseWriter, res handlers.Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.Code)
	if res.Body != nil {
		if err := json.NewEncoder(w).Encode(res.Body); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
	if res.Code == http.StatusInternalServerError {
		slog.Error("internal error", "error", res.Error.Error())
	}
}

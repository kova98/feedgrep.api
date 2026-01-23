package main

import (
	"context"
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/jmoiron/sqlx"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"

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

	db.SetMaxOpenConns(90)                 // Max 90 connections (under Postgres limit of 100)
	db.SetMaxIdleConns(25)                 // Keep 25 idle connections ready
	db.SetConnMaxLifetime(5 * time.Minute) // Recycle connections every 5 minutes
	db.SetConnMaxIdleTime(1 * time.Minute) // Close idle connections after 1 minute

	if err := data.RunMigrations(db.DB, embedMigrations); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	usersRepo := repos.NewUserRepo(db)
	users := handlers.NewUserHandler(usersRepo)
	hello := handlers.NewHelloHandler()
	keycloakClient := gocloak.NewClient(config.Config.KeycloakURL)
	auth = handlers.NewAuthHandler(keycloakClient)
	go auth.StartTokenTicker()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /hello", public(hello.GetHello))
	mux.HandleFunc("POST /hello", private(hello.PostHello))

	mux.HandleFunc("POST /users/init", private(users.InitializeUser))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigCh
		slog.Info("Shutting down...")
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

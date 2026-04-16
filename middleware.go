package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/handlers"
)

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

func private(handler handlers.Handler) http.Handler {
	return withMetrics(withAuth(handlerAdapter(handler)))
}

func privateHTTP(handler http.HandlerFunc) http.Handler {
	return withMetrics(withAuth(handler))
}

func public(handler handlers.Handler) http.Handler {
	return withMetrics(handlerAdapter(handler))
}

func handlerAdapter(handler handlers.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := handler(w, r)
		writeResult(w, res)
	})
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

func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := time.Now()
		ww := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		elapsedMs := time.Since(ts).Milliseconds()
		slog.Debug("req", "method", r.Method, "path", r.URL.Path, "code", ww.status, "elapsed", elapsedMs)
		apiMonitor.Request(r, ww.status, ts)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

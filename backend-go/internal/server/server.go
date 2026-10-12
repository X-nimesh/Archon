// Package server wires the HTTP routes together.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/archon/backend-go/internal/auth"
	"github.com/archon/backend-go/internal/cors"
	"github.com/archon/backend-go/internal/httpx"
	"github.com/archon/backend-go/internal/secure"
)

// New builds the fully wired http.Handler.
func New(authHandler *auth.Handler, authApp *auth.App, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	// Method-prefixed patterns come from Go 1.22's enhanced ServeMux, so no
	// third-party router is needed.
	mux.HandleFunc("GET /", root)
	mux.HandleFunc("GET /health", health)

	mux.HandleFunc("POST /auth/signup", authHandler.Signup)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.Handle("GET /auth/me", authApp.RequireAuth(http.HandlerFunc(authHandler.Me)))

	var handler http.Handler = mux

	handler = requestLogger(handler)
	handler = cors.Middleware(allowedOrigins, handler)
	handler = secure.Headers(handler)

	return handler
}

func root(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, "Hello World!")
}

func health(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// statusRecorder captures the status code so the logger can report it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		// Never log auth request bodies or headers: they carry credentials and
		// bearer tokens.
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

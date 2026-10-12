// Command api is the Archon backend HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/archon/backend-go/internal/auth"
	"github.com/archon/backend-go/internal/config"
	"github.com/archon/backend-go/internal/db"
	"github.com/archon/backend-go/internal/password"
	"github.com/archon/backend-go/internal/server"
	"github.com/archon/backend-go/internal/session"
	"github.com/archon/backend-go/internal/throttle"
	"github.com/archon/backend-go/internal/user"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required (see backend-go/.env.example)")
	}

	if cfg.EphemeralKey {
		slog.Warn("JWT_SECRET is not set - using a random dev secret. Sessions will not survive a restart.")
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── database ────────────────────────────────────────────────────────────
	startupCtx, cancelStartup := context.WithTimeout(rootCtx, 60*time.Second)
	defer cancelStartup()

	pool, err := db.Connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	slog.Info("db.connected")

	if cfg.RunMigrations {
		applied, err := db.Migrate(startupCtx, pool)
		if err != nil {
			return err
		}
		slog.Info("db.migrated", "applied", applied)
	}

	// ── wiring ──────────────────────────────────────────────────────────────
	users := user.NewStore(pool)
	tokens := session.NewStore(pool)
	issuer := session.NewIssuer(cfg.JWTSecret, cfg.AccessTTL, "archon-api")
	limiter := throttle.NewLimiter(pool, cfg.ThrottleWindow, cfg.IPFailures)

	app := auth.New(auth.Options{
		Users:        users,
		Tokens:       tokens,
		Issuer:       issuer,
		Limiter:      limiter,
		RefreshTTL:   cfg.RefreshTTL,
		PasswordArgs: argon2Params(),
	})

	handler := server.New(auth.NewHandler(app, cfg.TrustedProxy), app, cfg.CORSOrigins)

	srv := &http.Server{
		Addr:              cfg.Host + ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Background maintenance: drop rows that can no longer affect a decision.
	go reap(rootCtx, limiter, tokens)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "cors", cfg.CORSOrigins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-rootCtx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

// reap runs the periodic cleanup that keeps the auth tables from growing
// without bound. Runs until ctx is cancelled.
func reap(ctx context.Context, limiter *throttle.Limiter, tokens *session.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if n, err := limiter.Purge(runCtx); err != nil {
				slog.Error("reap.attempts_failed", "error", err)
			} else if n > 0 {
				slog.Info("reap.attempts", "deleted", n)
			}

			if n, err := tokens.DeleteExpired(runCtx); err != nil {
				slog.Error("reap.tokens_failed", "error", err)
			} else if n > 0 {
				slog.Info("reap.tokens", "deleted", n)
			}

			cancel()
		}
	}
}

// argon2Params reads Argon2 cost settings from the environment so they can be
// tuned against real hardware without a rebuild.
func argon2Params() password.Params {
	return password.Params{
		Memory:      uint32(envUint("ARGON2_MEMORY_KIB", uint(password.DefaultParams.Memory))),
		Iterations:  uint32(envUint("ARGON2_ITERATIONS", uint(password.DefaultParams.Iterations))),
		Parallelism: uint8(envUint("ARGON2_PARALLELISM", uint(password.DefaultParams.Parallelism))),
		SaltLength:  password.DefaultParams.SaltLength,
		KeyLength:   password.DefaultParams.KeyLength,
	}
}

func envUint(key string, fallback uint) uint {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 32)
		if err == nil && parsed > 0 {
			return uint(parsed)
		}
	}

	return fallback
}

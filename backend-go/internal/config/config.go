// Package config loads runtime configuration from the environment.
package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved application configuration.
type Config struct {
	Port string
	Host string

	JWTSecret    []byte
	EphemeralKey bool // true when JWTSecret was randomly generated

	CORSOrigins []string

	DatabaseURL     string
	RunMigrations   bool
	DBMaxConns      int32
	TrustedProxy    bool
	ShutdownTimeout time.Duration

	// Token lifetimes.
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	// Brute-force policy.
	ThrottleWindow    time.Duration
	IPFailures        int
	AccountFailures   int
	MaxFailedAttempts int
	LockoutDuration   time.Duration
}

// Load reads configuration from the environment, first applying any values
// found in the given .env file.
//
// Precedence: real environment variables always win over .env entries, which
// is what lets docker-compose override a checked-in default.
func Load(envFile string) (*Config, error) {
	if err := applyDotEnv(envFile); err != nil {
		return nil, err
	}

	secret, ephemeral, err := resolveJWTSecret()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:            envOr("PORT", "3001"),
		Host:            envOr("HOST", "0.0.0.0"),
		JWTSecret:       secret,
		EphemeralKey:    ephemeral,
		CORSOrigins:     splitList(envOr("CORS_ORIGIN", "http://localhost:3000")),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RunMigrations:   envBool("RUN_MIGRATIONS", true),
		DBMaxConns:      int32(envInt("DB_MAX_CONNS", 10)),
		TrustedProxy:    envBool("TRUST_PROXY", false),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		AccessTTL:  envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL: envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),

		ThrottleWindow:    envDuration("THROTTLE_WINDOW", 15*time.Minute),
		IPFailures:        envInt("THROTTLE_IP_FAILURES", 20),
		MaxFailedAttempts: envInt("MAX_FAILED_ATTEMPTS", 8),
		LockoutDuration:   envDuration("LOCKOUT_DURATION", 15*time.Minute),
	}, nil
}

// IsProduction reports whether the process should refuse dev-only behaviour.
func (c *Config) IsProduction() bool {
	return envOr("NODE_ENV", "development") == "production"
}

// resolveJWTSecret returns the signing key.
//
// In production a missing secret is fatal — falling back would boot the service
// with a guessable key. Elsewhere we generate a random one so the server runs
// without setup, at the cost of invalidating sessions on restart.
func resolveJWTSecret() ([]byte, bool, error) {
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		return []byte(secret), false, nil
	}

	if envOr("NODE_ENV", "development") == "production" {
		return nil, false, fmt.Errorf(
			"JWT_SECRET is required when NODE_ENV=production (generate one with: openssl rand -base64 32)")
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, false, fmt.Errorf("generate ephemeral JWT secret: %w", err)
	}

	return secret, true, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}

	return out
}

// applyDotEnv loads KEY=VALUE pairs into the environment without overwriting
// variables that are already set. Malformed lines are skipped, so a stray .env
// can never stop the server from booting.
func applyDotEnv(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}

	return nil
}

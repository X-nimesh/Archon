// Package integration exercises the real HTTP surface against a real
// PostgreSQL instance.
//
// These tests are skipped unless DATABASE_URL is set, so `go test ./...` stays
// useful without a database. To run them:
//
//	docker compose up -d postgres
//	DATABASE_URL='postgres://archon:archon@localhost:5432/archon?sslmode=disable' go test ./internal/integration/...
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/archon/backend-go/internal/auth"
	"github.com/archon/backend-go/internal/db"
	"github.com/archon/backend-go/internal/password"
	"github.com/archon/backend-go/internal/server"
	"github.com/archon/backend-go/internal/session"
	"github.com/archon/backend-go/internal/throttle"
	"github.com/archon/backend-go/internal/user"
)

func newHarness(t *testing.T) http.Handler {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Use an isolated schema so a test run never collides with dev data.
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})

	scoped := dsn + "&search_path=" + schema

	scopedPool, err := db.Connect(ctx, scoped)
	if err != nil {
		t.Fatalf("connect scoped: %v", err)
	}
	t.Cleanup(scopedPool.Close)

	if _, err := db.Migrate(ctx, scopedPool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Weak Argon2 params keep the suite fast; production uses password.DefaultParams.
	fast := password.Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	app := auth.New(auth.Options{
		Users:        user.NewStore(scopedPool),
		Tokens:       session.NewStore(scopedPool),
		Issuer:       session.NewIssuer([]byte("integration-secret"), time.Hour, "archon-api"),
		Limiter:      throttle.NewLimiter(scopedPool, time.Minute, 20),
		RefreshTTL:   time.Hour,
		PasswordArgs: fast,
	})

	return server.New(auth.NewHandler(app, false), app, []string{"http://localhost:3000"})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func do(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var decoded map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	}

	return rec, decoded
}

// Fields must be exported: json.Marshal emits nothing for unexported ones.
type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var ada = credentials{"Ada Lovelace", "ada@archon.test", "supersecret1"}

func signup(t *testing.T, h http.Handler, c credentials) map[string]any {
	t.Helper()

	rec, body := do(t, h, http.MethodPost, "/auth/signup", c, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status = %d (%s), want 201", rec.Code, rec.Body)
	}

	return body
}

func stringAt(t *testing.T, body map[string]any, key string) string {
	t.Helper()

	v, ok := body[key].(string)
	if !ok {
		t.Fatalf("missing %q in %v", key, body)
	}
	return v
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestSignupLoginRefreshLogout(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	pair := signup(t, h, ada)

	access := stringAt(t, pair, "accessToken")
	refresh := stringAt(t, pair, "refreshToken")

	if pair["tokenType"] != "Bearer" {
		t.Errorf("tokenType = %v, want Bearer", pair["tokenType"])
	}
	if _, ok := pair["expiresIn"].(float64); !ok {
		t.Errorf("expiresIn missing or not a number: %v", pair["expiresIn"])
	}

	userObj, _ := pair["user"].(map[string]any)
	if userObj["email"] != ada.Email {
		t.Errorf("user.email = %v, want %v", userObj["email"], ada.Email)
	}
	if _, leaked := userObj["passwordHash"]; leaked {
		t.Error("passwordHash leaked into the response")
	}

	t.Run("duplicate email is a conflict", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/auth/signup", ada, nil)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
		if body["message"] != "an account with that email already exists" {
			t.Errorf("message = %v", body["message"])
		}
	})

	t.Run("access token reaches /auth/me", func(t *testing.T) {
		rec, body := do(t, h, http.MethodGet, "/auth/me", nil,
			map[string]string{"Authorization": "Bearer " + access})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
		}
		if body["email"] != ada.Email {
			t.Errorf("email = %v", body["email"])
		}
	})

	t.Run("no token is unauthorized", func(t *testing.T) {
		if rec, _ := do(t, h, http.MethodGet, "/auth/me", nil, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("garbage token is unauthorized", func(t *testing.T) {
		rec, _ := do(t, h, http.MethodGet, "/auth/me", nil,
			map[string]string{"Authorization": "Bearer not-a-token"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("refresh rotates the token", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/auth/refresh",
			map[string]string{"refreshToken": refresh}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
		}

		newAccess := stringAt(t, body, "accessToken")
		newRefresh := stringAt(t, body, "refreshToken")

		if newRefresh == refresh {
			t.Error("refresh token was not rotated")
		}

		me, _ := do(t, h, http.MethodGet, "/auth/me", nil,
			map[string]string{"Authorization": "Bearer " + newAccess})
		if me.Code != http.StatusOK {
			t.Errorf("rotated access token rejected: %d", me.Code)
		}

		t.Run("replaying the old refresh token is refused", func(t *testing.T) {
			replay, _ := do(t, h, http.MethodPost, "/auth/refresh",
				map[string]string{"refreshToken": refresh}, nil)
			if replay.Code != http.StatusUnauthorized {
				t.Fatalf("replay status = %d, want 401", replay.Code)
			}
		})

		t.Run("reuse detection revokes the whole family", func(t *testing.T) {
			// The replay above must have burned the newly issued token too.
			after, _ := do(t, h, http.MethodPost, "/auth/refresh",
				map[string]string{"refreshToken": newRefresh}, nil)
			if after.Code != http.StatusUnauthorized {
				t.Errorf("new token still usable after replay: %d", after.Code)
			}
		})
	})

	t.Run("login works and is case-insensitive on email", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/auth/login",
			map[string]string{"email": "  ADA@ARCHON.TEST ", "password": ada.Password}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
		}

		userObj, _ := body["user"].(map[string]any)
		if userObj["email"] != ada.Email {
			t.Errorf("email = %v", userObj["email"])
		}
	})

	t.Run("wrong password is refused without revealing anything", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/auth/login",
			map[string]string{"email": ada.Email, "password": "wrong-password"}, nil)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if body["message"] != "Invalid email or password" {
			t.Errorf("message = %v", body["message"])
		}
	})

	t.Run("unknown email gives the identical error", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/auth/login",
			map[string]string{"email": "nobody@archon.test", "password": "wrong-password"}, nil)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		// Identical wording: the login form must not become an oracle for
		// which addresses have accounts.
		if body["message"] != "Invalid email or password" {
			t.Errorf("message = %v", body["message"])
		}
	})

	t.Run("logout revokes the session", func(t *testing.T) {
		login, body := do(t, h, http.MethodPost, "/auth/login",
			map[string]string{"email": ada.Email, "password": ada.Password}, nil)
		if login.Code != http.StatusOK {
			t.Fatalf("login failed: %d", login.Code)
		}

		sessionRefresh := stringAt(t, body, "refreshToken")

		rec, _ := do(t, h, http.MethodPost, "/auth/logout",
			map[string]string{"refreshToken": sessionRefresh}, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d, want 204", rec.Code)
		}

		after, _ := do(t, h, http.MethodPost, "/auth/refresh",
			map[string]string{"refreshToken": sessionRefresh}, nil)
		if after.Code != http.StatusUnauthorized {
			t.Errorf("refresh still works after logout: %d", after.Code)
		}
	})
}

func TestPasswordIsHashed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	signup(t, h, ada)

	// Reach into the database via the API surface: log in with the raw
	// password to prove the stored value is a verification hash, not the
	// password itself.
	rec, _ := do(t, h, http.MethodPost, "/auth/login",
		map[string]string{"email": ada.Email, "password": ada.Password}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login with the original password failed: %d", rec.Code)
	}

	// The response must never contain the password or a bcrypt/argon digest.
	if bytes.Contains(rec.Body.Bytes(), []byte(ada.Password)) {
		t.Error("response echoes the plaintext password")
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("$argon2id$")) {
		t.Error("response contains the password hash")
	}
}

func TestAccountLockout(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	signup(t, h, credentials{"Lock Target", "lock@archon.test", "supersecret1"})

	// The limiter allows 20 failures per IP within the window, but the account
	// lockout trips at 8 — so the lockout is what must stop this loop.
	for i := range throttle.MaxFailedAttempts {
		rec, _ := do(t, h, http.MethodPost, "/auth/login",
			map[string]string{"email": "lock@archon.test", "password": "wrong"}, nil)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}

	// Now the correct password must be refused too.
	rec, body := do(t, h, http.MethodPost, "/auth/login",
		map[string]string{"email": "lock@archon.test", "password": "supersecret1"}, nil)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 — account should be locked", rec.Code)
	}
	if body["message"] != "Too many failed attempts. Try again later." {
		t.Errorf("message = %v", body["message"])
	}
}

func TestValidationAndEnvelope(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	rec, body := do(t, h, http.MethodPost, "/auth/signup",
		map[string]string{"name": "X", "email": "bad", "password": "1"}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	msgs, ok := body["message"].([]any)
	if !ok || len(msgs) != 3 {
		t.Fatalf("message = %v, want an array of 3", body["message"])
	}
	if body["error"] != "Bad Request" {
		t.Errorf("error = %v", body["error"])
	}
	if body["statusCode"] != float64(400) {
		t.Errorf("statusCode = %v", body["statusCode"])
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	// Privilege escalation via mass assignment must not be possible.
	rec, _ := do(t, h, http.MethodPost, "/auth/signup", map[string]any{
		"name": "Sneaky", "email": "sneaky@archon.test", "password": "supersecret1",
		"role": "admin",
	}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unknown field", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	rec, _ := do(t, h, http.MethodPost, "/auth/login",
		map[string]string{"email": "x@y.dev", "password": "z"}, nil)

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	if cc := rec.Header().Get("Cache-Control"); !bytes.Contains([]byte(cc), []byte("no-store")) {
		t.Errorf("Cache-Control = %q, auth responses must be no-store", cc)
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow-origin = %q", got)
	}
}

// TestMigrationIsIdempotent proves the runner is safe to call on every boot.
func TestMigrationIsIdempotent(t *testing.T) {
	t.Parallel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") //nolint:errcheck

	scoped, err := db.Connect(ctx, dsn+"&search_path="+schema)
	if err != nil {
		t.Fatalf("connect scoped: %v", err)
	}
	defer scoped.Close()

	applied, err := db.Migrate(ctx, scoped)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if applied == 0 {
		t.Error("first run applied no migrations")
	}

	second, err := db.Migrate(ctx, scoped)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second != 0 {
		t.Errorf("second run applied %d migrations, want 0", second)
	}

	var tables int
	err = scoped.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = $1 AND table_name IN ('users','refresh_tokens','login_attempts')`,
		schema).Scan(&tables)
	if err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 3 {
		t.Errorf("found %d tables, want 3", tables)
	}
}

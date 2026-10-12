// Package throttle implements per-IP brute-force protection.
//
// Scope of responsibility, deliberately narrow:
//
//   - This package limits how fast ONE host can attempt logins.
//   - Per-ACCOUNT protection is the lockout stored on the user row
//     (users.failed_login_count / users.locked_until), not a second counter
//     here.
//
// Having both would be redundant -- they would trip at the same threshold and
// race to produce the error, making it unclear which one the user hit. Splitting
// them also covers the two attacks properly: per-IP stops one host spraying many
// accounts, and the account lockout stops a botnet hammering one account from
// many IPs.
//
// Attempts are recorded in the `login_attempts` table under both scopes, so the
// data is there for an audit view without needing a second write path.
//
// Identifiers are SHA-256 hashed before storage: a leaked table should not
// enumerate which IPs or emails have visited the login page.
package throttle

import (
	"context"
	"fmt"
	"time"

	"github.com/archon/backend-go/internal/db"
)

// Scope selects which counter a row belongs to.
type Scope string

const (
	// ScopeIP is the counter this package enforces.
	ScopeIP Scope = "ip"
	// ScopeAccount is recorded for audit but not enforced here; the account
	// lockout is what protects a single account.
	ScopeAccount Scope = "account"
)

// DefaultIPFailures is how many failed logins one address may accumulate inside
// the window. Tuned to allow a human mistyping a password repeatedly, not to
// stop an attacker who is also rate limited at the edge.
const DefaultIPFailures = 20

// DefaultWindow is the sliding period failures are counted over.
const DefaultWindow = 15 * time.Minute

// Limiter counts recent failures in Postgres.
type Limiter struct {
	pool    *db.Pool
	window  time.Duration
	ipLimit int
}

// NewLimiter builds a limiter.
func NewLimiter(pool *db.Pool, window time.Duration, ipLimit int) *Limiter {
	if window <= 0 {
		window = DefaultWindow
	}
	if ipLimit <= 0 {
		ipLimit = DefaultIPFailures
	}

	return &Limiter{pool: pool, window: window, ipLimit: ipLimit}
}

// Verdict is the outcome of a throttle check.
type Verdict struct {
	Blocked bool
	// Reason is which counter tripped. Only for server-side logging — never
	// surface it to the client, or it tells an attacker which limit they hit.
	Reason  string
	RetryIn time.Duration
}

// Check reports whether a login attempt from this address may proceed.
func (l *Limiter) Check(ctx context.Context, ip string) (Verdict, error) {
	if ip == "" {
		return Verdict{}, nil
	}

	const query = `
		SELECT count(*), max(attempted_at)
		FROM login_attempts
		WHERE scope = $1
		  AND key = $2
		  AND succeeded = FALSE
		  AND attempted_at > now() - make_interval(secs => $3)`

	var (
		failures int
		lastAt   *time.Time
	)

	if err := l.pool.QueryRow(ctx, query, string(ScopeIP), HashKey(string(ScopeIP), ip), l.window.Seconds()).
		Scan(&failures, &lastAt); err != nil {
		return Verdict{}, fmt.Errorf("throttle check: %w", err)
	}

	if failures < l.ipLimit {
		return Verdict{}, nil
	}

	retryIn := l.window
	if lastAt != nil {
		if remaining := l.window - time.Since(*lastAt); remaining > 0 {
			retryIn = remaining
		} else {
			retryIn = 0
		}
	}

	return Verdict{Blocked: true, Reason: "ip_limit", RetryIn: retryIn}, nil
}

// Record appends one login attempt.
func (l *Limiter) Record(ctx context.Context, scope Scope, key string, succeeded bool) error {
	if key == "" {
		return nil
	}

	const query = `
		INSERT INTO login_attempts (scope, key, succeeded)
		VALUES ($1, $2, $3)`

	if _, err := l.pool.Exec(ctx, query, string(scope), HashKey(string(scope), key), succeeded); err != nil {
		return fmt.Errorf("record attempt: %w", err)
	}

	return nil
}

// Purge deletes attempts older than the window. Call on a timer so the table
// does not grow without bound.
func (l *Limiter) Purge(ctx context.Context) (int64, error) {
	const query = `DELETE FROM login_attempts WHERE attempted_at < now() - make_interval(secs => $1)`

	tag, err := l.pool.Exec(ctx, query, (l.window * 2).Seconds())
	if err != nil {
		return 0, fmt.Errorf("purge attempts: %w", err)
	}

	return tag.RowsAffected(), nil
}

// Lockout policy for a single account.
//
// This is the per-account counterpart to the per-IP limiter. It lives on the
// user row rather than in login_attempts because it must follow the account
// across every IP that targets it.
const (
	MaxFailedAttempts = 8
	LockoutDuration   = 15 * time.Minute
)

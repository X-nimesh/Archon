// Package session issues and rotates the access/refresh token pair.
//
// Design:
//
//   - The **access token** is a stateless signed JWT (default 15 min). It is
//     never stored, so authenticating a normal request costs no database round
//     trip.
//   - The **refresh token** is a 256-bit random opaque string (default 30 days).
//     Only its SHA-256 hash is stored. Presenting it rotates it: the old row is
//     marked used and a new one is issued.
//   - **Reuse detection**: if an already-rotated refresh token is presented, it
//     is assumed stolen and the user's entire token family is revoked. This is
//     the property that makes rotation worth the complexity.
//
// See the `refresh_tokens` table in the root AGENTS.md.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/archon/backend-go/internal/db"
)

var (
	// ErrNotFound means the presented refresh token is unknown.
	ErrNotFound = errors.New("refresh token not found")
	// ErrReused means an already-rotated token was replayed. Treat the whole
	// family as compromised.
	ErrReused = errors.New("refresh token reuse detected")
	// ErrExpired means the token aged out.
	ErrExpired = errors.New("refresh token expired")
	// ErrStale means the user's credentials changed after this token was
	// minted, so the token is no longer honoured.
	ErrStale = errors.New("refresh token issued before credentials changed")
)

// Store persists refresh tokens in PostgreSQL.
type Store struct {
	pool *db.Pool
}

// NewStore builds a Postgres-backed refresh token store.
func NewStore(pool *db.Pool) *Store {
	return &Store{pool: pool}
}

// Issue mints a new refresh token for a user and stores its hash.
//
// Returns the plaintext token, which is never persisted and never logged.
func (s *Store) Issue(
	ctx context.Context,
	userID string,
	credentialsChangedAt time.Time,
	ttl time.Duration,
	userAgent, ip string,
) (string, time.Time, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().UTC().Add(ttl)

	const query = `
		INSERT INTO refresh_tokens
			(user_id, token_hash, credentials_changed_at, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))`

	_, err = s.pool.Exec(ctx, query, userID, hash, credentialsChangedAt, expiresAt, userAgent, ip)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert refresh token: %w", err)
	}

	return raw, expiresAt, nil
}

// Consume validates a refresh token and rotates it, returning the owning user.
//
// On ErrReused the caller MUST revoke every token for that user — that is the
// whole point of rotation.
func (s *Store) Consume(ctx context.Context, raw string, newTTL time.Duration, userAgent, ip string) (string, string, time.Time, error) {
	hash := HashToken(raw)

	const selectQuery = `
		SELECT user_id, expires_at, rotated_at, revoked_at, credentials_changed_at
		FROM refresh_tokens
		WHERE token_hash = $1`

	var (
		userID                string
		expiresAt             time.Time
		rotatedAt, revokedAt  *time.Time
		tokenCredentialsStamp time.Time
	)

	err := s.pool.QueryRow(ctx, selectQuery, hash).Scan(
		&userID, &expiresAt, &rotatedAt, &revokedAt, &tokenCredentialsStamp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", time.Time{}, ErrNotFound
	}
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("select refresh token: %w", err)
	}

	// Replay of a consumed token: assume theft, burn the family.
	if rotatedAt != nil || revokedAt != nil {
		return userID, "", time.Time{}, ErrReused
	}

	if time.Now().UTC().After(expiresAt) {
		return userID, "", time.Time{}, ErrExpired
	}

	// Password change invalidates every token minted beforehand.
	var currentStamp time.Time
	if err := s.pool.QueryRow(ctx, `SELECT credentials_changed_at FROM users WHERE id = $1`, userID).
		Scan(&currentStamp); err != nil {
		return userID, "", time.Time{}, fmt.Errorf("read credentials_changed_at: %w", err)
	}

	if tokenCredentialsStamp.Before(currentStamp) {
		return userID, "", time.Time{}, ErrStale
	}

	// Rotate: mark used and mint the replacement in one transaction, so a crash
	// cannot leave the user with both tokens dead or both alive.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return userID, "", time.Time{}, fmt.Errorf("begin rotation: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const markQuery = `
		UPDATE refresh_tokens SET rotated_at = now()
		WHERE token_hash = $1 AND rotated_at IS NULL AND revoked_at IS NULL`
	tag, err := tx.Exec(ctx, markQuery, hash)
	if err != nil {
		return userID, "", time.Time{}, fmt.Errorf("mark rotated: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Another request rotated it between our SELECT and UPDATE.
		return userID, "", time.Time{}, ErrReused
	}

	newRaw, newHash, err := newToken()
	if err != nil {
		return userID, "", time.Time{}, err
	}

	newExpiry := time.Now().UTC().Add(newTTL)

	const insertQuery = `
		INSERT INTO refresh_tokens
			(user_id, token_hash, credentials_changed_at, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))`

	if _, err := tx.Exec(ctx, insertQuery, userID, newHash, currentStamp, newExpiry, userAgent, ip); err != nil {
		return userID, "", time.Time{}, fmt.Errorf("insert rotated token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return userID, "", time.Time{}, fmt.Errorf("commit rotation: %w", err)
	}

	return userID, newRaw, newExpiry, nil
}

// RevokeAll burns every live token for a user. Called on logout, on password
// change, and as the response to refresh-token reuse.
func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`

	if _, err := s.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("revoke tokens: %w", err)
	}

	return nil
}

// DeleteExpired removes rows that can no longer be used. Call periodically.
func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	const query = `
		DELETE FROM refresh_tokens
		WHERE expires_at < now() - INTERVAL '7 days'`

	tag, err := s.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete expired tokens: %w", err)
	}

	return tag.RowsAffected(), nil
}

// newToken returns a plaintext token and the hash to store.
func newToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(buf)

	return raw, HashToken(raw), nil
}

// HashToken derives the stored representation of a refresh token.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

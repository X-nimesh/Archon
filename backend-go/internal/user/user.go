// Package user holds the user model and its persistence.
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/archon/backend-go/internal/db"
)

var (
	// ErrEmailTaken is returned when a signup collides with an existing account.
	ErrEmailTaken = errors.New("an account with that email already exists")
	// ErrNotFound is returned when a lookup matches no user.
	ErrNotFound = errors.New("user not found")
	// ErrAccountDisabled is returned when a login hits a closed account.
	ErrAccountDisabled = errors.New("this account is disabled")
)

// User mirrors a row of the `users` table.
//
// Field mapping is documented in the root AGENTS.md — update that table when
// this struct changes.
type User struct {
	ID                   string
	Name                 string
	Email                string
	PasswordHash         *string
	CredentialsChangedAt time.Time
	FailedLoginCount     int
	LockedUntil          *time.Time
	LastLoginAt          *time.Time
	DisabledAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// IsLocked reports whether the account is inside its lockout window at now.
func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}

// CanAuthenticate reports whether the account may attempt a login at all.
func (u *User) CanAuthenticate() bool {
	return u.DisabledAt == nil && u.PasswordHash != nil
}

const userColumns = `
	id, name, email, password_hash, credentials_changed_at,
	failed_login_count, locked_until, last_login_at, disabled_at,
	created_at, updated_at`

// Repository is the persistence seam the auth package depends on.
type Repository interface {
	Create(ctx context.Context, name, email, passwordHash string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	RecordLoginSuccess(ctx context.Context, id string) error
	RecordLoginFailure(ctx context.Context, id string, lockAfter int, lockFor time.Duration) (FailureState, error)
	UpdatePasswordHash(ctx context.Context, id, passwordHash string) error
}

// FailureState is the account's brute-force state after a failed attempt.
type FailureState struct {
	Count       int
	LockedUntil time.Time
	// Locked reports whether this attempt tripped the lockout.
	Locked bool
}

var _ Repository = (*Store)(nil)

// Store is the PostgreSQL-backed Repository.
type Store struct {
	pool *db.Pool
}

// NewStore returns a Postgres-backed repository.
func NewStore(pool *db.Pool) *Store {
	return &Store{pool: pool}
}

// NormalizeEmail is the canonical form used for every lookup, matching the
// CITEXT normalisation the database performs.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Store) Create(ctx context.Context, name, email, passwordHash string) (*User, error) {
	const query = `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING ` + userColumns

	row := s.pool.QueryRow(ctx, query, strings.TrimSpace(name), NormalizeEmail(email), passwordHash)

	u, err := scanUser(row)
	if err != nil {
		// The unique index is the authority on duplicates, not a prior SELECT,
		// so two concurrent signups cannot both win.
		if db.IsUniqueViolation(err) {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == "users_email_key" {
				return nil, ErrEmailTaken
			}
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return u, nil
}

func (s *Store) FindByEmail(ctx context.Context, email string) (*User, error) {
	const query = `SELECT ` + userColumns + ` FROM users WHERE email = $1`

	return scanUser(s.pool.QueryRow(ctx, query, NormalizeEmail(email)))
}

func (s *Store) FindByID(ctx context.Context, id string) (*User, error) {
	const query = `SELECT ` + userColumns + ` FROM users WHERE id = $1`

	return scanUser(s.pool.QueryRow(ctx, query, id))
}

// RecordLoginSuccess clears the brute-force counters.
func (s *Store) RecordLoginSuccess(ctx context.Context, id string) error {
	const query = `
		UPDATE users
		SET failed_login_count = 0,
		    locked_until       = NULL,
		    last_login_at      = now()
		WHERE id = $1`

	if _, err := s.pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("record login success: %w", err)
	}

	return nil
}

// RecordLoginFailure increments the failure counter and applies the lockout in
// the same statement, so a concurrent attempt cannot slip past the threshold.
func (s *Store) RecordLoginFailure(ctx context.Context, id string, lockAfter int, lockFor time.Duration) (FailureState, error) {
	const query = `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
		    locked_until = CASE
		        WHEN failed_login_count + 1 >= $2
		        THEN now() + make_interval(secs => $3)
		        ELSE locked_until
		    END
		WHERE id = $1
		RETURNING failed_login_count, locked_until`

	var (
		count       int
		lockedUntil *time.Time
	)

	err := s.pool.QueryRow(ctx, query, id, lockAfter, lockFor.Seconds()).Scan(&count, &lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return FailureState{}, ErrNotFound
	}
	if err != nil {
		return FailureState{}, fmt.Errorf("record login failure: %w", err)
	}

	state := FailureState{Count: count}
	if lockedUntil != nil {
		state.LockedUntil = *lockedUntil
		state.Locked = state.LockedUntil.After(time.Now())
	}

	return state, nil
}

// UpdatePasswordHash swaps in a stronger hash and invalidates every session
// that predates the change by advancing credentials_changed_at.
func (s *Store) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	const query = `
		UPDATE users
		SET password_hash          = $2,
		    credentials_changed_at = now(),
		    failed_login_count      = 0,
		    locked_until            = NULL
		WHERE id = $1`

	if _, err := s.pool.Exec(ctx, query, id, passwordHash); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	return nil
}

// rowScanner unifies *pgx.Row and pgx.Rows for the shared scan helper.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var u User

	err := row.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.CredentialsChangedAt,
		&u.FailedLoginCount,
		&u.LockedUntil,
		&u.LastLoginAt,
		&u.DisabledAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}

	return &u, nil
}

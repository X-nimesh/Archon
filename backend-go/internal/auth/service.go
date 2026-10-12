// Package auth implements signup, login, refresh, and logout.
//
// The backend is the only authority on identity. This package owns credential
// verification; token minting lives in internal/session and password storage in
// internal/password.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/archon/backend-go/internal/password"
	"github.com/archon/backend-go/internal/session"
	"github.com/archon/backend-go/internal/throttle"
	"github.com/archon/backend-go/internal/user"
)

// Errors surfaced to the HTTP layer.
//
// ErrInvalidCredentials is intentionally vague: revealing whether an email is
// registered turns the login form into an account-enumeration oracle.
var (
	ErrInvalidCredentials = errors.New("Invalid email or password")
	ErrAccountLocked      = errors.New("Too many failed attempts. Try again later.")
	ErrRateLimited        = errors.New("Too many requests. Try again later.")
	ErrDisabled           = errors.New("This account is disabled")
	ErrBadRefreshToken    = errors.New("Invalid or expired session")
	ErrInternal           = errors.New("Something went wrong")
)

// App is the auth application service.
type App struct {
	users    user.Repository
	tokens   *session.Store
	issuer   *session.Issuer
	limiter  *throttle.Limiter
	pwParams password.Params

	refreshTTL time.Duration
}

// Options configures an App.
type Options struct {
	Users        user.Repository
	Tokens       *session.Store
	Issuer       *session.Issuer
	Limiter      *throttle.Limiter
	RefreshTTL   time.Duration
	PasswordArgs password.Params
}

// New builds an App.
func New(opts Options) *App {
	if opts.RefreshTTL <= 0 {
		opts.RefreshTTL = 30 * 24 * time.Hour
	}
	if opts.PasswordArgs.Memory == 0 {
		opts.PasswordArgs = password.DefaultParams
	}

	return &App{
		users:      opts.Users,
		tokens:     opts.Tokens,
		issuer:     opts.Issuer,
		limiter:    opts.Limiter,
		pwParams:   opts.PasswordArgs,
		refreshTTL: opts.RefreshTTL,
	}
}

// PublicUser is the safe projection returned by the API.
type PublicUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"createdAt"`
}

func toPublic(u *user.User) PublicUser {
	return PublicUser{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Format(time.RFC3339Nano),
	}
}

// TokenPair is the response to signup, login, and refresh.
type TokenPair struct {
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken"`
	ExpiresIn    int        `json:"expiresIn"` // access-token lifetime, seconds
	TokenType    string     `json:"tokenType"` // always "Bearer"
	User         PublicUser `json:"user"`
}

// SignupInput is the decoded POST /auth/signup body.
type SignupInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginInput is the decoded POST /auth/login body.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshInput is the decoded POST /auth/refresh body.
type RefreshInput struct {
	RefreshToken string `json:"refreshToken"`
}

// Signup creates an account and signs the new user in.
func (a *App) Signup(ctx context.Context, in SignupInput, meta ClientMeta) (TokenPair, error) {
	hash, err := password.Hash(in.Password, a.pwParams)
	if err != nil {
		return TokenPair{}, err
	}

	account, err := a.users.Create(ctx, in.Name, in.Email, hash)
	if err != nil {
		return TokenPair{}, err
	}

	pair, err := a.issue(ctx, account, meta)
	if err != nil {
		return TokenPair{}, err
	}

	slog.Info("auth.signup", "user_id", account.ID)

	return pair, nil
}

// Login verifies credentials, enforcing rate limits and account lockout.
func (a *App) Login(ctx context.Context, in LoginInput, meta ClientMeta) (TokenPair, error) {
	email := user.NormalizeEmail(in.Email)

	// Throttle before touching the accounts table, so a flood is cheap. This is
	// the per-IP limit; per-account protection is the lockout below.
	verdict, err := a.limiter.Check(ctx, meta.IP)
	if err != nil {
		return TokenPair{}, err
	}
	if verdict.Blocked {
		slog.Warn("auth.throttled", "scope", verdict.Reason, "ip", meta.IP)
		return TokenPair{}, ErrRateLimited
	}

	account, err := a.users.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return TokenPair{}, err
	}
	if errors.Is(err, user.ErrNotFound) {
		account = nil
	}

	// Always run a comparison, even with no account, so response time does not
	// reveal whether the email is registered.
	verified := a.verifyPassword(account, in.Password)

	if !verified {
		a.recordFailure(ctx, account, meta)
		return TokenPair{}, ErrInvalidCredentials
	}

	if account.IsLocked(time.Now()) {
		return TokenPair{}, ErrAccountLocked
	}

	if account.DisabledAt != nil {
		return TokenPair{}, ErrDisabled
	}

	if err := a.users.RecordLoginSuccess(ctx, account.ID); err != nil {
		return TokenPair{}, err
	}

	_ = a.limiter.Record(ctx, throttle.ScopeIP, meta.IP, true)
	_ = a.limiter.Record(ctx, throttle.ScopeAccount, email, true)

	// Opportunistically upgrade hashes made with weaker parameters, so raising
	// the cost later does not require a password reset campaign.
	a.upgradeHash(ctx, account, in.Password)

	pair, err := a.issue(ctx, account, meta)
	if err != nil {
		return TokenPair{}, err
	}

	slog.Info("auth.login", "user_id", account.ID)

	return pair, nil
}

// verifyPassword runs the Argon2id comparison against the account's stored hash,
// or against a decoy when there is no usable account. Returns false in every
// failure case; the caller never learns which one applied.
func (a *App) verifyPassword(account *user.User, candidate string) bool {
	stored := decoyHash()

	if account != nil && account.PasswordHash != nil {
		stored = *account.PasswordHash
	}

	ok, err := password.Verify(candidate, stored)
	if err != nil {
		// A malformed stored hash is a data bug, not a wrong password. Log it,
		// but report a generic failure so we do not leak its existence.
		slog.Error("auth.hash_invalid", "error", err)
		return false
	}

	return ok && account != nil && account.PasswordHash != nil
}

func (a *App) recordFailure(ctx context.Context, account *user.User, meta ClientMeta) {
	_ = a.limiter.Record(ctx, throttle.ScopeIP, meta.IP, false)

	email := ""
	if account != nil {
		email = account.Email
	}
	_ = a.limiter.Record(ctx, throttle.ScopeAccount, email, false)

	if account == nil {
		return
	}

	state, err := a.users.RecordLoginFailure(ctx, account.ID, throttle.MaxFailedAttempts, throttle.LockoutDuration)
	if err != nil {
		slog.Error("auth.record_failure", "error", err)
		return
	}

	if state.Locked {
		slog.Warn("auth.locked", "user_id", account.ID, "failures", state.Count)
	}
}

// upgradeHash re-hashes the password with current params if the stored hash
// used weaker settings. Best-effort: a failure here must not fail the login.
func (a *App) upgradeHash(ctx context.Context, account *user.User, plaintext string) {
	if account.PasswordHash == nil {
		return
	}
	if !password.NeedsRehash(*account.PasswordHash, a.pwParams) {
		return
	}

	stronger, err := password.Hash(plaintext, a.pwParams)
	if err != nil {
		slog.Error("auth.hash_upgrade_failed", "error", err)
		return
	}

	if err := a.users.UpdatePasswordHash(ctx, account.ID, stronger); err != nil {
		slog.Error("auth.hash_upgrade_failed", "error", err)
	}
}

// Refresh rotates a refresh token and mints a new access token.
//
// On reuse detection the user's entire token family is revoked.
func (a *App) Refresh(ctx context.Context, in RefreshInput, meta ClientMeta) (TokenPair, error) {
	userID, rotated, _, err := a.tokens.Consume(ctx, in.RefreshToken, a.refreshTTL, meta.UserAgent, meta.IP)

	if err != nil {
		switch {
		case errors.Is(err, session.ErrReused):
			// Assume theft: burn every live session for this user.
			if revErr := a.tokens.RevokeAll(ctx, userID); revErr != nil {
				slog.Error("auth.revoke_failed", "error", revErr)
			}
			slog.Warn("auth.refresh_reuse", "user_id", userID)

		case errors.Is(err, session.ErrExpired),
			errors.Is(err, session.ErrStale),
			errors.Is(err, session.ErrNotFound):
			slog.Info("auth.refresh_rejected", "user_id", userID, "reason", err.Error())
		}

		return TokenPair{}, ErrBadRefreshToken
	}

	account, err := a.users.FindByID(ctx, userID)
	if err != nil {
		return TokenPair{}, ErrBadRefreshToken
	}

	if account.DisabledAt != nil {
		_ = a.tokens.RevokeAll(ctx, userID)
		return TokenPair{}, ErrDisabled
	}

	access, accessExpiry, err := a.issuer.Issue(account.ID)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: rotated,
		ExpiresIn:    int(time.Until(accessExpiry).Seconds()),
		TokenType:    "Bearer",
		User:         toPublic(account),
	}, nil
}

// Logout revokes every live session for the user.
func (a *App) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	userID, _, _, err := a.tokens.Consume(ctx, refreshToken, a.refreshTTL, "", "")
	if err != nil && !errors.Is(err, session.ErrNotFound) &&
		!errors.Is(err, session.ErrExpired) && !errors.Is(err, session.ErrReused) {
		return err
	}

	if userID == "" {
		return nil
	}

	return a.tokens.RevokeAll(ctx, userID)
}

// RefreshTTL is the refresh-token lifetime, for cookie max-age purposes.
func (a *App) RefreshTTL() time.Duration { return a.refreshTTL }

// AccessTTL is the access-token lifetime.
func (a *App) AccessTTL() time.Duration { return a.issuer.TTL() }

func (a *App) issue(ctx context.Context, account *user.User, meta ClientMeta) (TokenPair, error) {
	access, accessExpiry, err := a.issuer.Issue(account.ID)
	if err != nil {
		return TokenPair{}, err
	}

	refresh, _, err := a.tokens.Issue(
		ctx, account.ID, account.CredentialsChangedAt, a.refreshTTL, meta.UserAgent, meta.IP)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(time.Until(accessExpiry).Seconds()),
		TokenType:    "Bearer",
		User:         toPublic(account),
	}, nil
}

// decoyHash is an Argon2id hash of a value nobody knows. Verifying against it
// keeps the "no such account" path as slow as the real one.
var decoyHash = sync.OnceValue(func() string {
	h, err := password.Hash("archon-decoy-not-a-real-password", password.DefaultParams)
	if err != nil {
		panic("auth: cannot build decoy hash: " + err.Error())
	}
	return h
})

// ClientMeta is the request context worth recording for abuse tracking.
type ClientMeta struct {
	IP        string
	UserAgent string
}

// ClientMetaFrom extracts ClientMeta from a request.
//
// X-Forwarded-For is only consulted when trustProxy is set, because that header
// is attacker-controlled and would otherwise let anyone bypass per-IP limits
// by sending a fake value.
func ClientMetaFrom(r *http.Request, trustProxy bool) ClientMeta {
	return ClientMeta{
		IP:        clientIP(r, trustProxy),
		UserAgent: strings.TrimSpace(r.UserAgent()),
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first, _, found := strings.Cut(fwd, ","); found {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(fwd)
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/archon/backend-go/internal/httpx"
	"github.com/archon/backend-go/internal/session"
	"github.com/archon/backend-go/internal/user"
)

// contextKey is unexported so no other package can collide with it.
type contextKey struct{}

var accountKey contextKey

// AccountFrom returns the authenticated account placed in the request context
// by RequireAuth.
func AccountFrom(ctx context.Context) (*user.User, bool) {
	account, ok := ctx.Value(accountKey).(*user.User)
	return account, ok
}

// Authenticate resolves the bearer token on a request.
//
// Returns (nil, nil) when no Authorization header was sent, which lets
// handlers choose whether anonymous access is allowed. A header that is present
// but invalid is always an error.
func (a *App) Authenticate(r *http.Request) (*user.User, error) {
	token, ok := bearerToken(r)
	if !ok {
		return nil, nil
	}

	userID, err := a.issuer.Verify(token)
	if err != nil {
		return nil, err
	}

	account, err := a.users.FindByID(r.Context(), userID)
	if err != nil {
		// Token was validly signed but the account is gone or disabled.
		return nil, session.ErrInvalidAccessToken
	}

	if account.DisabledAt != nil {
		return nil, user.ErrAccountDisabled
	}

	return account, nil
}

// RequireAuth rejects requests without a valid access token, and attaches the
// resolved account to the request context.
//
// Unlike the stateless JWT alone, this re-checks the database, so disabling an
// account or deleting it takes effect immediately rather than at token expiry.
func (a *App) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, err := a.Authenticate(r)

		switch {
		case err != nil:
			httpx.WriteError(w, http.StatusUnauthorized, "Invalid or expired session")
			return
		case account == nil:
			httpx.WriteError(w, http.StatusUnauthorized, "Missing bearer token")
			return
		}

		ctx := context.WithValue(r.Context(), accountKey, account)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}

	return token, true
}

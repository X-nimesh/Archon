package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/archon/backend-go/internal/httpx"
	"github.com/archon/backend-go/internal/session"
	"github.com/archon/backend-go/internal/user"
)

// Handler exposes the auth routes over HTTP.
type Handler struct {
	app *App
	// trustProxy controls whether X-Forwarded-For is believed for rate limiting.
	trustProxy bool
}

// NewHandler builds the auth HTTP handler.
func NewHandler(app *App, trustProxy bool) *Handler {
	return &Handler{app: app, trustProxy: trustProxy}
}

// Signup handles POST /auth/signup.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var in SignupInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return
	}

	if msgs := ValidateSignup(in); len(msgs) > 0 {
		httpx.WriteValidationError(w, msgs)
		return
	}

	result, err := h.app.Signup(r.Context(), in, ClientMetaFrom(r, h.trustProxy))
	if err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			httpx.WriteError(w, http.StatusConflict, user.ErrEmailTaken.Error())
			return
		}
		slog.Error("auth.signup_failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, ErrInternal.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, result)
}

// Login handles POST /auth/login. Responds 200 — signing in creates no new
// resource as far as the caller is concerned.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return
	}

	if msgs := ValidateLogin(in); len(msgs) > 0 {
		httpx.WriteValidationError(w, msgs)
		return
	}

	result, err := h.app.Login(r.Context(), in, ClientMetaFrom(r, h.trustProxy))
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

// Refresh handles POST /auth/refresh, rotating the refresh token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var in RefreshInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return
	}

	if in.RefreshToken == "" {
		httpx.WriteError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}

	result, err := h.app.Refresh(r.Context(), in, ClientMetaFrom(r, h.trustProxy))
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

// Logout handles POST /auth/logout.
//
// Always answers 204, even for an unknown token: telling the caller whether a
// token was real leaks nothing useful but does make the endpoint easier to
// probe.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var in RefreshInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return
	}

	if err := h.app.Logout(r.Context(), in.RefreshToken); err != nil {
		slog.Error("auth.logout_failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, ErrInternal.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /auth/me and is wrapped by RequireAuth.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	account, ok := AccountFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Invalid or expired session")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toPublic(account))
}

// writeAuthError maps domain errors to status codes.
//
// Bad credentials, lockout, and rate limiting all return 401/429 with a vague
// body on purpose — see ErrInvalidCredentials.
func (h *Handler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
	case errors.Is(err, ErrAccountLocked):
		httpx.WriteError(w, http.StatusTooManyRequests, ErrAccountLocked.Error())
	case errors.Is(err, ErrRateLimited):
		httpx.WriteError(w, http.StatusTooManyRequests, ErrRateLimited.Error())
	case errors.Is(err, ErrDisabled):
		httpx.WriteError(w, http.StatusForbidden, ErrDisabled.Error())
	case errors.Is(err, ErrBadRefreshToken), errors.Is(err, session.ErrInvalidAccessToken):
		httpx.WriteError(w, http.StatusUnauthorized, ErrBadRefreshToken.Error())
	default:
		slog.Error("auth.failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, ErrInternal.Error())
	}
}

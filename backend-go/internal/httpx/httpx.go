// Package httpx holds small HTTP helpers shared across handlers.
//
// The error envelope here is deliberately identical to the original NestJS
// backend:
//
//	{"message": string|string[], "error": "Bad Request", "statusCode": 400}
//
// The frontend parses that shape in lib/api.ts, so keeping it byte-compatible
// means the UI works unchanged.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse is the single error shape every failing route returns.
type ErrorResponse struct {
	Message    any    `json:"message"`
	Error      string `json:"error"`
	StatusCode int    `json:"statusCode"`
}

// statusText maps a status code to the short label used in `error`.
func statusText(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusTooManyRequests:
		return "Too Many Requests"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	default:
		return http.StatusText(code)
	}
}

// WriteJSON serialises v with the given status.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "error", err)
	}
}

// WriteError sends a single-message error, e.g. "Invalid email or password".
func WriteError(w http.ResponseWriter, code int, message string) {
	WriteJSON(w, code, ErrorResponse{
		Message:    message,
		Error:      statusText(code),
		StatusCode: code,
	})
}

// WriteValidationError sends a multi-message error, matching the shape Nest's
// ValidationPipe produced when several fields failed at once.
func WriteValidationError(w http.ResponseWriter, messages []string) {
	WriteJSON(w, http.StatusBadRequest, ErrorResponse{
		Message:    messages,
		Error:      statusText(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
	})
}

// DecodeJSON reads a JSON request body, rejecting unknown fields the same way
// Nest's `whitelist` + `forbidNonWhitelisted` pipes did.
//
// Also caps the body size: without this a single request can make the server
// buffer an arbitrary amount of memory.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	const maxBody = 8 << 10 // 8 KiB is far more than any auth payload needs

	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		WriteValidationError(w, []string{err.Error()})
		return err
	}

	return nil
}

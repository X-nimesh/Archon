package auth

import (
	"net/http"
	"net/http/httptest"
)

func newRequest(remoteAddr, forwardedFor string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	r.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		r.Header.Set("X-Forwarded-For", forwardedFor)
	}
	return r
}

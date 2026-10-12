package session

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidAccessToken covers expired, tampered, and malformed access tokens.
var ErrInvalidAccessToken = errors.New("invalid or expired access token")

// Claims is the access-token payload.
//
// Deliberately minimal — a token that travels in a header on every request
// should not carry email addresses or anything else that is expensive to
// rotate. The subject id is all the API needs.
type Claims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// Issuer mints and verifies short-lived access tokens.
type Issuer struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// NewIssuer builds an access-token issuer.
func NewIssuer(secret []byte, ttl time.Duration, issuer string) *Issuer {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Issuer{secret: secret, ttl: ttl, issuer: issuer}
}

// TTL exposes the configured lifetime so handlers can set cookie max-age to match.
func (i *Issuer) TTL() time.Duration { return i.ttl }

// Issue returns a signed access token for a user id.
func (i *Issuer) Issue(userID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(i.ttl)

	claims := Claims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			// Give the id a random jti so two tokens minted in the same second
			// for the same user are still distinguishable in a log.
			ID: newID(),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

// Verify parses and validates an access token, returning the subject.
//
// The algorithm is pinned to HS256 so an attacker cannot downgrade the token to
// `alg: none` and have it accepted.
func (i *Issuer) Verify(token string) (string, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return i.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil || !parsed.Valid {
		return "", ErrInvalidAccessToken
	}

	// A refresh token is opaque, but if one is ever pasted into the access slot
	// it must not authenticate. Reject anything not explicitly typed.
	if claims.TokenType != "access" {
		return "", ErrInvalidAccessToken
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return "", ErrInvalidAccessToken
	}

	return subject, nil
}

func newID() string {
	raw, _, err := newToken()
	if err != nil {
		return ""
	}
	return raw
}

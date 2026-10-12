package session

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "unit-test-secret"

func newIssuer(ttl time.Duration) *Issuer {
	return NewIssuer([]byte(testSecret), ttl, "archon-api")
}

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Parallel()
	issuer := newIssuer(time.Hour)

	token, expiresAt, err := issuer.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if token == "" {
		t.Fatal("empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiry %v is not in the future", expiresAt)
	}

	subject, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if subject != "user-123" {
		t.Errorf("subject = %q, want user-123", subject)
	}
}

func TestAccessTokenRejectsTampering(t *testing.T) {
	t.Parallel()
	issuer := newIssuer(time.Hour)

	token, _, err := issuer.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	cases := map[string]string{
		"garbage":           "not-a-token",
		"empty":             "",
		"signature flipped": token[:len(token)-4] + "AAAA",
		"payload swapped":   swapPayload(t, token),
	}

	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := issuer.Verify(candidate); err == nil {
				t.Errorf("expected rejection for %s", name)
			}
		})
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	t.Parallel()

	issued := newIssuer(time.Hour)
	other := NewIssuer([]byte("a-different-secret"), time.Hour, "archon-api")

	token, _, err := issued.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := other.Verify(token); err == nil {
		t.Error("token verified under the wrong secret")
	}
}

func TestAccessTokenRejectsUnsignedToken(t *testing.T) {
	t.Parallel()
	issuer := newIssuer(time.Hour)

	// alg=none downgrade: the classic JWT attack.
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "attacker",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := issuer.Verify(unsigned); err == nil {
		t.Error("an unsigned token was accepted")
	}
}

func TestAccessTokenRequiresExpiry(t *testing.T) {
	t.Parallel()

	// A token with no exp claim must be refused, otherwise a leaked token is
	// valid forever.
	forever, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TokenType:        "access",
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123"},
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := newIssuer(time.Hour).Verify(forever); err == nil {
		t.Error("a token without an expiry was accepted")
	}
}

func TestAccessTokenRejectsWrongIssuer(t *testing.T) {
	t.Parallel()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "some-other-service",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := newIssuer(time.Hour).Verify(token); err == nil {
		t.Error("a token from another issuer was accepted")
	}
}

func TestAccessTokenExpires(t *testing.T) {
	t.Parallel()

	// Negative TTL is clamped to the 15-minute default, so build an explicitly
	// expired token instead of waiting.
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "archon-api",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := newIssuer(time.Hour).Verify(expired); err == nil {
		t.Error("an expired token was accepted")
	}
}

func TestRefreshHashIsStableAndOpaque(t *testing.T) {
	t.Parallel()

	const raw = "a-refresh-token"

	first, second := HashToken(raw), HashToken(raw)
	if first != second {
		t.Error("HashToken is not deterministic")
	}
	if strings.Contains(first, raw) {
		t.Error("stored hash contains the plaintext token")
	}
	if HashToken("different") == first {
		t.Error("different tokens collided")
	}
}

func swapPayload(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape: %s", token)
	}

	// Keep the original signature, swap in a different subject/signature.
	other, _, err := newIssuer(time.Hour).Issue("someone-else")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	otherParts := strings.Split(other, ".")

	return parts[0] + "." + otherParts[1] + "." + parts[2]
}

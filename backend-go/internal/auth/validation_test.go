package auth

import (
	"strings"
	"testing"
)

func TestValidateSignup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   SignupInput
		want string
	}{
		{"name too short", SignupInput{Name: "X", Email: "a@b.dev", Password: "supersecret1"}, MsgNameTooShort},
		{"name too long", SignupInput{Name: strings.Repeat("x", 81), Email: "a@b.dev", Password: "supersecret1"}, MsgNameTooLong},
		{"bad email", SignupInput{Name: "Ada", Email: "not-an-email", Password: "supersecret1"}, MsgInvalidEmail},
		{"email with display name", SignupInput{Name: "Ada", Email: "Ada <ada@b.dev>", Password: "supersecret1"}, MsgInvalidEmail},
		{"email without domain dot", SignupInput{Name: "Ada", Email: "ada@localhost", Password: "supersecret1"}, MsgInvalidEmail},
		{"password too short", SignupInput{Name: "Ada", Email: "a@b.dev", Password: "short"}, MsgPwTooShort},
		{"password too long", SignupInput{Name: "Ada", Email: "a@b.dev", Password: strings.Repeat("x", 129)}, MsgPwTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msgs := ValidateSignup(tc.in)
			if !contains(msgs, tc.want) {
				t.Errorf("messages = %v, want it to contain %q", msgs, tc.want)
			}
		})
	}

	t.Run("accepts a valid signup", func(t *testing.T) {
		in := SignupInput{Name: "Ada", Email: "ada@archon.dev", Password: "supersecret1"}
		if msgs := ValidateSignup(in); len(msgs) > 0 {
			t.Errorf("unexpected messages: %v", msgs)
		}
	})

	t.Run("reports every problem at once", func(t *testing.T) {
		msgs := ValidateSignup(SignupInput{Name: "X", Email: "bad", Password: "1"})
		if len(msgs) != 3 {
			t.Errorf("want 3 messages, got %v", msgs)
		}
	})

	t.Run("counts runes not bytes", func(t *testing.T) {
		// A 2-character name of multi-byte runes must pass the length check.
		in := SignupInput{Name: "नि", Email: "a@b.dev", Password: "supersecret1"}
		if msgs := ValidateSignup(in); len(msgs) > 0 {
			t.Errorf("multi-byte name rejected: %v", msgs)
		}
	})
}

func TestValidateLogin(t *testing.T) {
	t.Parallel()

	t.Run("accepts valid input", func(t *testing.T) {
		if msgs := ValidateLogin(LoginInput{Email: "ada@archon.dev", Password: "x"}); len(msgs) > 0 {
			t.Errorf("unexpected messages: %v", msgs)
		}
	})

	t.Run("rejects an empty password", func(t *testing.T) {
		msgs := ValidateLogin(LoginInput{Email: "ada@archon.dev"})
		if !contains(msgs, MsgPwRequired) {
			t.Errorf("messages = %v", msgs)
		}
	})

	t.Run("caps password length before hashing", func(t *testing.T) {
		// Argon2's memory cost scales with input size, so an unbounded login
		// password would be a memory-exhaustion vector.
		msgs := ValidateLogin(LoginInput{
			Email:    "ada@archon.dev",
			Password: strings.Repeat("x", MaxPasswordLength+1),
		})
		if !contains(msgs, MsgPwTooLong) {
			t.Errorf("messages = %v, want %q", msgs, MsgPwTooLong)
		}
	})

	t.Run("does not impose a minimum on login passwords", func(t *testing.T) {
		// Signup requires 8+, but an account created before that rule (or via
		// another path) must still be able to log in with a shorter one.
		if msgs := ValidateLogin(LoginInput{Email: "ada@archon.dev", Password: "a"}); len(msgs) > 0 {
			t.Errorf("short legacy password rejected at login: %v", msgs)
		}
	})
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	t.Run("uses RemoteAddr by default", func(t *testing.T) {
		r := newRequest("203.0.113.9:5000", "")

		if got := clientIP(r, false); got != "203.0.113.9" {
			t.Errorf("ip = %q, want 203.0.113.9", got)
		}
	})

	t.Run("ignores X-Forwarded-For unless trusted", func(t *testing.T) {
		// Otherwise anyone could bypass per-IP rate limits by sending a header.
		r := newRequest("10.0.0.1:5000", "1.2.3.4")

		if got := clientIP(r, false); got != "10.0.0.1" {
			t.Errorf("ip = %q, want the socket address", got)
		}
	})

	t.Run("uses the left-most forwarded entry when trusted", func(t *testing.T) {
		r := newRequest("10.0.0.1:5000", "1.2.3.4, 5.6.7.8")

		if got := clientIP(r, true); got != "1.2.3.4" {
			t.Errorf("ip = %q, want 1.2.3.4", got)
		}
	})

	t.Run("handles a bare address with no port", func(t *testing.T) {
		r := newRequest("not-an-ip", "")

		if got := clientIP(r, false); got != "not-an-ip" {
			t.Errorf("ip = %q", got)
		}
	})
}

func contains(msgs []string, want string) bool {
	for _, m := range msgs {
		if m == want {
			return true
		}
	}
	return false
}

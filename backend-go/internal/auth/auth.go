package auth

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Validation messages are kept identical to the original NestJS implementation so
// the UI renders the same wording regardless of which backend is running.
//
// Changing one of these strings is a user-visible change — see the contract
// note in the root AGENTS.md.
const (
	MsgInvalidEmail = "Please enter a valid email address."
	MsgNameTooShort = "Name must be at least 2 characters long"
	MsgNameTooLong  = "Name must be at most 80 characters long"
	MsgPwTooShort   = "Password must be at least 8 characters long"
	MsgPwTooLong    = "Password must be at most 128 characters long"
	MsgPwRequired   = "Password is required"
)

// MaxPasswordLength bounds what Argon2id is asked to hash.
//
// Argon2 deliberately consumes memory proportional to the *password*, so an
// unbounded input is a memory-exhaustion vector. This cap is enforced in the
// handler path and is the same one bcrypt's implementations use.
const MaxPasswordLength = 128

// ValidateSignup checks a signup payload and returns every failure at once,
// mirroring the array-of-messages response the frontend already handles.
func ValidateSignup(in SignupInput) []string {
	var errs []string

	nameLen := utf8.RuneCountInString(strings.TrimSpace(in.Name))
	switch {
	case nameLen < 2:
		errs = append(errs, MsgNameTooShort)
	case nameLen > 80:
		errs = append(errs, MsgNameTooLong)
	}

	if !looksLikeEmail(in.Email) {
		errs = append(errs, MsgInvalidEmail)
	}

	switch pwLen := utf8.RuneCountInString(in.Password); {
	case pwLen < 1:
		errs = append(errs, MsgPwRequired)
	case pwLen < 8:
		errs = append(errs, MsgPwTooShort)
	case pwLen > MaxPasswordLength:
		errs = append(errs, MsgPwTooLong)
	}

	return errs
}

// ValidateLogin checks a login payload.
func ValidateLogin(in LoginInput) []string {
	var errs []string

	if !looksLikeEmail(in.Email) {
		errs = append(errs, MsgInvalidEmail)
	}

	if in.Password == "" {
		errs = append(errs, MsgPwRequired)
	} else if utf8.RuneCountInString(in.Password) > MaxPasswordLength {
		// Bounded for the same reason as signup: Argon2id's memory cost scales
		// with input length, so this must be capped before hashing.
		errs = append(errs, MsgPwTooLong)
	}

	return errs
}

// looksLikeEmail is stricter than net/mail's parser on the points that matter
// for account identity.
func looksLikeEmail(email string) bool {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 254 {
		return false
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return false
	}

	// ParseAddress happily accepts display names ("Ada <ada@x.com>"). Require a
	// bare address so the stored value is exactly what was typed.
	if addr.Address != trimmed || addr.Name != "" {
		return false
	}

	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return false
	}

	domain := trimmed[at+1:]

	return strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".")
}

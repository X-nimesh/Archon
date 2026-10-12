// Package password implements Argon2id password hashing.
//
// OWASP's current first recommendation for password storage. Argon2id is
// memory-hard, so an attacker with GPUs or ASICs gains far less per dollar
// than they would against bcrypt.
//
// Hashes are stored in PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<b64salt>$<b64digest>
//
// Embedding the parameters means they can be raised later without invalidating
// existing hashes: Hasher.Verify transparently accepts any well-formed PHC
// string and re-hashes on next successful login if it used weaker settings.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params configures the Argon2id cost.
//
// Defaults follow the OWASP "19 MiB, t=2, p=1" baseline, nudged to 64 MiB
// because this is a small API where memory per hash is cheap. Tune against
// real hardware: the target is roughly 100–250 ms per hash.
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams are the settings used for newly created hashes.
var DefaultParams = Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// ErrInvalidHash means the stored value is not a well-formed PHC string. It is
// a data-integrity problem, not a wrong-password problem, and the two are never
// conflated in the caller's response.
var ErrInvalidHash = errors.New("invalid password hash format")

const (
	algorithm = "argon2id"
	version   = 19
)

// Hash derives a new hash from password using the given params.
func Hash(password string, params Params) (string, error) {
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	digest := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)

	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		algorithm,
		version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// Verify reports whether password matches the encoded PHC hash.
//
// Uses argon2's constant-time comparison internally.
func Verify(password, encoded string) (bool, error) {
	parsed, err := parse(encoded)
	if err != nil {
		return false, err
	}

	other := argon2.IDKey([]byte(password), parsed.salt, parsed.iterations, parsed.memory, parsed.parallelism, uint32(len(parsed.digest)))

	return subtle.ConstantTimeCompare(other, parsed.digest) == 1, nil
}

// NeedsRehash reports whether an encoded hash was produced with settings weaker
// than the ones currently configured. Callers upgrade the stored hash on the
// next successful login when this is true.
func NeedsRehash(encoded string, current Params) bool {
	parsed, err := parse(encoded)
	if err != nil {
		return true
	}

	return parsed.memory < current.Memory ||
		parsed.iterations < current.Iterations ||
		parsed.parallelism < current.Parallelism ||
		uint32(len(parsed.digest)) < current.KeyLength
}

type phc struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	digest      []byte
}

func parse(encoded string) (phc, error) {
	var out phc

	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, digest]
	if len(parts) != 6 || parts[0] != "" {
		return out, ErrInvalidHash
	}

	if parts[1] != algorithm {
		return out, fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidHash, parts[1])
	}

	if _, err := fmt.Sscanf(parts[2], "v=%d", new(int)); err != nil {
		return out, fmt.Errorf("%w: bad version", ErrInvalidHash)
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &out.memory, &out.iterations, &out.parallelism); err != nil {
		return out, fmt.Errorf("%w: bad params", ErrInvalidHash)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return out, fmt.Errorf("%w: bad salt", ErrInvalidHash)
	}

	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return out, fmt.Errorf("%w: bad digest", ErrInvalidHash)
	}

	out.salt, out.digest = salt, digest

	if len(out.digest) == 0 || out.memory == 0 || out.iterations == 0 || out.parallelism == 0 {
		return out, ErrInvalidHash
	}

	return out, nil
}

package throttle

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashKey derives the stored form of a throttle key.
//
// Scoped with the counter name so the same value tracked under `ip` and
// `account` produces two unrelated rows — otherwise an attacker who picks an
// email matching a seen IP could collide counters.
func HashKey(scope, key string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + key))
	return hex.EncodeToString(sum[:])
}

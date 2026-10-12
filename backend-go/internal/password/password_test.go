package password

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	t.Parallel()

	const plaintext = "correct horse battery staple"

	hash, err := Hash(plaintext, DefaultParams)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	t.Run("accepts the right password", func(t *testing.T) {
		ok, err := Verify(plaintext, hash)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !ok {
			t.Error("expected the correct password to verify")
		}
	})

	t.Run("rejects a wrong password", func(t *testing.T) {
		ok, err := Verify("wrong password", hash)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if ok {
			t.Error("wrong password verified")
		}
	})

	t.Run("rejects an empty password", func(t *testing.T) {
		ok, _ := Verify("", hash)
		if ok {
			t.Error("empty password verified")
		}
	})

	t.Run("never contains the plaintext", func(t *testing.T) {
		if strings.Contains(hash, plaintext) {
			t.Error("hash leaks the password")
		}
	})
}

func TestSaltIsUniquePerHash(t *testing.T) {
	t.Parallel()

	first, err := Hash("same-password", DefaultParams)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	second, err := Hash("same-password", DefaultParams)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if first == second {
		t.Error("two hashes of the same password are identical — salt is not random")
	}

	// Both must still verify, which is what makes a per-hash salt invisible to
	// the attacker holding only the hashes.
	for _, h := range []string{first, second} {
		if ok, err := Verify("same-password", h); err != nil || !ok {
			t.Errorf("Verify(%s) = %v, %v", h, ok, err)
		}
	}
}

func TestPHCFormat(t *testing.T) {
	t.Parallel()

	hash, err := Hash("pw", DefaultParams)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// $argon2id$v=19$m=...,t=...,p=...$salt$digest
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("want 6 $-separated fields, got %d: %s", len(parts), hash)
	}

	if parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		t.Errorf("unexpected header: %s", hash)
	}

	if !strings.HasPrefix(parts[3], "m=") || !strings.Contains(parts[3], ",t=") || !strings.Contains(parts[3], ",p=") {
		t.Errorf("params field should carry m, t and p: %q", parts[3])
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"empty":              "",
		"not a phc string":   "hunter2",
		"missing fields":     "$argon2id$v=19$m=65536,t=3,p=2$salt",
		"wrong algorithm":    "$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"leading $ missing":  "argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"bad base64 salt":    "$argon2id$v=19$m=65536,t=3,p=2$!!!!$aGFzaA",
		"bad base64 digest":  "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!!",
		"zero iterations":    "$argon2id$v=19$m=65536,t=0,p=2$c2FsdA$aGFzaA",
		"empty digest field": "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$",
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Verify("anything", encoded); err == nil {
				t.Errorf("expected an error for %q", encoded)
			}
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	t.Parallel()

	t.Run("freshly hashed with current params needs no rehash", func(t *testing.T) {
		hash, err := Hash("pw", DefaultParams)
		if err != nil {
			t.Fatalf("Hash: %v", err)
		}
		if NeedsRehash(hash, DefaultParams) {
			t.Error("a hash made with current params should not need re-hashing")
		}
	})

	t.Run("weak params are flagged", func(t *testing.T) {
		weak := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

		hash, err := Hash("pw", weak)
		if err != nil {
			t.Fatalf("Hash: %v", err)
		}
		if !NeedsRehash(hash, DefaultParams) {
			t.Error("a weak hash should be flagged for upgrade")
		}
	})

	t.Run("malformed hash is flagged", func(t *testing.T) {
		if !NeedsRehash("not-a-hash", DefaultParams) {
			t.Error("malformed hash should be flagged so it gets replaced")
		}
	})
}

func TestDifferentParamsBothVerify(t *testing.T) {
	t.Parallel()

	// Simulates a params upgrade: the old hash still verifies under the new
	// params because the parameters travel inside the PHC string.
	old := Params{Memory: 16 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	hash, err := Hash("pw", old)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if ok, err := Verify("pw", hash); err != nil || !ok {
		t.Errorf("old-param hash failed to verify: %v, %v", ok, err)
	}

	if !NeedsRehash(hash, DefaultParams) {
		t.Error("old-param hash should be flagged for upgrade")
	}
}

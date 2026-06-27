package main

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	t.Run("basic validity", func(t *testing.T) {
		verifier, challenge, err := generatePKCE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if verifier == "" {
			t.Error("verifier should not be empty")
		}

		if challenge == "" {
			t.Error("challenge should not be empty")
		}
	})

	t.Run("verifier length according to RFC 7636", func(t *testing.T) {
		verifier, _, err := generatePKCE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// RFC 7636 section 4.1: code_verifier must be between 43 and 128 octets
		if len(verifier) < 43 || len(verifier) > 128 {
			t.Errorf("verifier length %d is not between 43 and 128", len(verifier))
		}
	})

	t.Run("verifier valid characters", func(t *testing.T) {
		verifier, _, err := generatePKCE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// RFC 7636 section 4.1: [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
		// The current implementation uses base64.RawURLEncoding which produces
		// [A-Z], [a-z], [0-9], "-", "_"
		validCharRegex := regexp.MustCompile(`^[A-Za-z0-9\-_]+$`)
		if !validCharRegex.MatchString(verifier) {
			t.Errorf("verifier contains invalid characters: %s", verifier)
		}
	})

	t.Run("challenge computation", func(t *testing.T) {
		verifier, challenge, err := generatePKCE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Calculate what the challenge should be
		h := sha256.New()
		h.Write([]byte(verifier))
		expectedChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

		if challenge != expectedChallenge {
			t.Errorf("challenge %s does not match expected computed challenge %s", challenge, expectedChallenge)
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		// Generate 100 values to ensure they are unique (checking randomness)
		generated := make(map[string]bool)
		for i := 0; i < 100; i++ {
			verifier, _, err := generatePKCE()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if generated[verifier] {
				t.Fatalf("generatePKCE produced duplicate verifier on iteration %d: %s", i, verifier)
			}
			generated[verifier] = true
		}
	})
}

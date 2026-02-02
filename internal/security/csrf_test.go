package security

import (
	"regexp"
	"testing"
)

func TestTokenManager_Generate(t *testing.T) {
	tm := NewTokenManager()

	token, err := tm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Token should be 64 characters (32 bytes * 2 hex chars per byte)
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Token should be valid hex string
	hexPattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if !hexPattern.MatchString(token) {
		t.Errorf("token = %s, want valid hex string", token)
	}
}

func TestTokenManager_Generate_Uniqueness(t *testing.T) {
	tm := NewTokenManager()

	token1, err := tm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	token2, err := tm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Tokens should be different (cryptographically random)
	if token1 == token2 {
		t.Error("Generate() produced identical tokens, want unique tokens")
	}
}

func TestTokenManager_Generate_MultipleTokens(t *testing.T) {
	tm := NewTokenManager()
	tokens := make(map[string]bool)

	// Generate 100 tokens and ensure none are duplicated
	for i := 0; i < 100; i++ {
		token, err := tm.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}

		if tokens[token] {
			t.Errorf("Generate() produced duplicate token on iteration %d", i)
		}
		tokens[token] = true
	}
}

package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var ErrInvalidToken = errors.New("invalid CSRF token")

// TokenManager handles CSRF token generation.
// Tokens are cryptographically random and stored server-side in the session.
// Verification is done through database lookup (not cryptographic signature).
type TokenManager struct{}

// NewTokenManager creates a new CSRF token manager.
func NewTokenManager() *TokenManager {
	return &TokenManager{}
}

// Generate creates a cryptographically secure random CSRF token (128 bits).
// The token is returned as a 64-character hex string.
func (tm *TokenManager) Generate() (string, error) {
	// Generate 32 random bytes (256 bits) for maximum entropy
	// This provides 256 bits of security against brute force attacks
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	// Convert to hex string for safe transmission and storage
	return hex.EncodeToString(randomBytes), nil
}

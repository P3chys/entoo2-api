package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateSecureToken generates a cryptographically secure random token
// of the specified byte length and returns it as a URL-safe base64 string
func GenerateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashToken hashes a token using SHA-256 for deterministic, lookupable storage.
// SHA-256 is appropriate here because tokens are high-entropy random values
// (not user-chosen passwords), so brute-force resistance from bcrypt is unnecessary.
func HashToken(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:]), nil
}

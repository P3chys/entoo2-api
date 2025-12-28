package utils

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
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

// HashToken hashes a token using bcrypt for secure storage in the database
func HashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyToken checks if a plain token matches a bcrypt-hashed token
// Uses constant-time comparison via bcrypt to prevent timing attacks
func VerifyToken(hashedToken, plainToken string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedToken), []byte(plainToken))
	return err == nil
}

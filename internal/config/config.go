package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	// Server
	Port    string
	GinMode string

	// Database
	DatabaseURL string

	// File Storage (local filesystem / Railway volume)
	StoragePath string

	// JWT
	JWTSecret        string
	JWTAccessExpiry  string
	JWTRefreshExpiry string

	// SMTP
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string

	// Resend (HTTP email API - used when SMTP is blocked)
	ResendAPIKey string

	// Application URLs
	AppURL string

	// Token Expiry
	EmailVerificationExpiry string
	PasswordResetExpiry     string

	// CORS
	CORSOrigins []string
}

func Load() *Config {
	return &Config{
		Port:    getEnv("PORT", "8000"),
		GinMode: getEnv("GIN_MODE", "release"),

		DatabaseURL: requireEnv("DATABASE_URL"),

		StoragePath: getEnv("STORAGE_PATH", "/data/uploads"),

		JWTSecret:        requireEnv("JWT_SECRET"),
		JWTAccessExpiry:  getEnv("JWT_ACCESS_EXPIRY", "15m"),
		JWTRefreshExpiry: getEnv("JWT_REFRESH_EXPIRY", "168h"),

		SMTPHost:      getEnv("SMTP_HOST", "localhost"),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUsername:  getEnv("SMTP_USERNAME", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", "noreply@localhost"),
		SMTPFromName:  getEnv("SMTP_FROM_NAME", "Entoo2 Portal"),
		ResendAPIKey:  getEnv("RESEND_API_KEY", ""),

		AppURL: getEnv("APP_URL", "http://localhost:5173"),

		EmailVerificationExpiry: getEnv("EMAIL_VERIFICATION_EXPIRY", "24h"),
		PasswordResetExpiry:     getEnv("PASSWORD_RESET_EXPIRY", "1h"),

		CORSOrigins: strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000"), ","),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("FATAL: Required environment variable %s is not set", key)
	}
	return value
}

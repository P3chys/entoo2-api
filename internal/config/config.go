package config

import (
	"os"
	"strings"
)

type Config struct {
	// Server
	Port    string
	GinMode string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// MinIO
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool

	// Meilisearch
	MeiliURL    string
	MeiliAPIKey string

	// Tika
	TikaURL string

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
		GinMode: getEnv("GIN_MODE", "debug"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://entoo2:entoo2_dev@localhost:5432/entoo2?sslmode=disable"),

		RedisURL: getEnv("REDIS_URL", "redis://:redis_dev@localhost:6379/0"),

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "documents"),
		MinIOUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",

		MeiliURL:    getEnv("MEILI_URL", "http://localhost:7700"),
		MeiliAPIKey: getEnv("MEILI_API_KEY", "dev_master_key_change_in_production"),

		TikaURL: getEnv("TIKA_URL", "http://localhost:9998"),

		JWTSecret:        getEnv("JWT_SECRET", "development_secret"),
		JWTAccessExpiry:  getEnv("JWT_ACCESS_EXPIRY", "15m"),
		JWTRefreshExpiry: getEnv("JWT_REFRESH_EXPIRY", "168h"),

		SMTPHost:      getEnv("SMTP_HOST", "localhost"),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUsername:  getEnv("SMTP_USERNAME", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", "noreply@localhost"),
		SMTPFromName:  getEnv("SMTP_FROM_NAME", "Entoo2 Portal"),

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

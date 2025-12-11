package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	log.Println("Database connected successfully")
	return db, nil
}

func RunMigrations(db *gorm.DB) error {
	log.Println("Running migrations...")

	// Import models here to avoid circular dependencies
	type User struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		Email        string    `gorm:"uniqueIndex;not null"`
		PasswordHash string    `gorm:"not null"`
		Role         string    `gorm:"type:varchar(20);default:'student'"`
		DisplayName  string    `gorm:"size:100"`
		Language     string    `gorm:"type:varchar(2);default:'cs'"`
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	type Semester struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		Name        string    `gorm:"not null"`
		Description string
		StartDate   string
		EndDate     string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	type Subject struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SemesterID  string    `gorm:"type:uuid;not null"`
		Name        string    `gorm:"not null"`
		Code        string    `gorm:"uniqueIndex"`
		Description string
		Credits     int
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	type Document struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null"`
		UserID      string    `gorm:"type:uuid;not null"`
		Name        string    `gorm:"not null"`
		Description string
		FilePath    string    `gorm:"not null"`
		FileSize    int64
		MimeType    string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	// Auto-migrate all models
	err := db.AutoMigrate(&User{}, &Semester{}, &Subject{}, &Document{})
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

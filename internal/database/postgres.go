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
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		NameCS     string    `gorm:"column:name_cs;size:100;not null"`
		NameEN     string    `gorm:"column:name_en;size:100;not null"`
		OrderIndex int       `gorm:"column:order_index;default:0"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type Subject struct {
		ID            string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SemesterID    string    `gorm:"type:uuid;not null"`
		NameCS        string    `gorm:"column:name_cs;size:200;not null"`
		NameEN        string    `gorm:"column:name_en;size:200;not null"`
		Code          string    `gorm:"size:10;uniqueIndex"`
		DescriptionCS string    `gorm:"column:description_cs;type:text"`
		DescriptionEN string    `gorm:"column:description_en;type:text"`
		Credits       int       `gorm:"default:0"`
		CreatedAt     time.Time
		UpdatedAt     time.Time
	}

	type SubjectTeacher struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null"`
		TeacherName string    `gorm:"size:200;not null"`
		TopicCS     string    `gorm:"size:300"`
		TopicEN     string    `gorm:"size:300"`
		CreatedAt   time.Time
	}

	type Document struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID    string    `gorm:"type:uuid;not null;index"`
		UploadedBy   string    `gorm:"type:uuid;not null;index"`
		Filename     string    `gorm:"size:255;not null"`
		OriginalName string    `gorm:"size:255;not null"`
		FileSize     int64     `gorm:"not null"`
		MimeType     string    `gorm:"size:100;not null"`
		MinIOPath    string    `gorm:"size:500;not null"`
		ContentText  string    `gorm:"type:text"`
		CreatedAt    time.Time `gorm:"index"`
	}

	type Activity struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		UserID       string    `gorm:"type:uuid;not null;index"`
		ActivityType string    `gorm:"type:varchar(50);not null;index"`
		SubjectID    *string   `gorm:"type:uuid;index"`
		DocumentID   *string   `gorm:"type:uuid;index"`
		Metadata     string    `gorm:"type:jsonb"`
		CreatedAt    time.Time `gorm:"index"`
	}

	// Auto-migrate all models
	err := db.AutoMigrate(&User{}, &Semester{}, &Subject{}, &SubjectTeacher{}, &Document{}, &Activity{})
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

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

	type Semester struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		NameCS     string    `gorm:"column:name_cs;size:100;not null"`
		OrderIndex int       `gorm:"column:order_index;default:0"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type Subject struct {
		ID            string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SemesterID    string    `gorm:"type:uuid;not null"`
		NameCS        string    `gorm:"column:name_cs;size:200;not null"`
		Code          string    `gorm:"size:10;uniqueIndex"`
		DescriptionCS string    `gorm:"column:description_cs;type:text"`
		Credits       int       `gorm:"default:0"`
		CreatedAt     time.Time
		UpdatedAt     time.Time
	}

	type SubjectTeacher struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null"`
		TeacherName string    `gorm:"size:200;not null"`
		TopicCS     string    `gorm:"size:300"`
		CreatedAt   time.Time
	}

	type DocumentCategory struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID  string    `gorm:"type:uuid;not null;index:idx_subject_type"`
		Type       string    `gorm:"size:20;not null;index:idx_subject_type"`
		NameCS     string    `gorm:"size:200;not null"`
		NameEN     string    `gorm:"size:200;not null"`
		OrderIndex int       `gorm:"not null;default:0;index:idx_order"`
		CreatedBy  string    `gorm:"type:uuid;not null"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type Document struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID    string    `gorm:"type:uuid;not null;index"`
		UploadedBy   string    `gorm:"type:uuid;not null;index"`
		AnswerID     *string   `gorm:"type:uuid;index"`
		Type         string    `gorm:"size:20;default:'other'"`
		CategoryID   *string   `gorm:"type:uuid;index"`
		Filename     string    `gorm:"size:255;not null"`
		OriginalName string    `gorm:"size:255;not null"`
		FileSize     int64     `gorm:"not null"`
		MimeType     string    `gorm:"size:100;not null"`
		MinIOPath    string    `gorm:"size:500;not null"`
		ContentText  string    `gorm:"type:text"`
		CreatedAt    time.Time `gorm:"index"`
	}

	type User struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		Email        string    `gorm:"uniqueIndex;not null"`
		PasswordHash string    `gorm:"not null"`
		Role         string    `gorm:"type:varchar(20);default:'student'"`
		DisplayName  string    `gorm:"size:100"`
		Language     string    `gorm:"type:varchar(2);default:'cs'"`
		CreatedAt    time.Time
		UpdatedAt    time.Time

		// Email Verification
		EmailVerified          bool       `gorm:"default:false"`
		EmailVerificationToken *string    `gorm:"size:255;index"`
		EmailVerificationSentAt *time.Time
		EmailVerifiedAt        *time.Time

		// Password Reset
		PasswordResetToken     *string    `gorm:"size:255;index"`
		PasswordResetSentAt    *time.Time
		PasswordResetExpiresAt *time.Time

		FavoriteSubjects  []Subject  `gorm:"many2many:user_favorite_subjects;"`
		FavoriteDocuments []Document `gorm:"many2many:user_favorite_documents;"`
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

	type Comment struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null"`
		UserID      string    `gorm:"type:uuid;not null"`
		Content     string    `gorm:"type:text;not null"`
		IsAnonymous bool      `gorm:"default:false"`
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	type Question struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null;index"`
		UserID      string    `gorm:"type:uuid;not null;index"`
		Content     string    `gorm:"type:text;not null"`
		IsAnonymous bool      `gorm:"default:false"`
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	type Answer struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		QuestionID string    `gorm:"type:uuid;not null;index"`
		UserID     string    `gorm:"type:uuid;not null;index"`
		Content    string    `gorm:"type:text;not null"`
		DocumentID *string   `gorm:"type:uuid;index"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type TeacherRating struct {
		ID               string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectTeacherID string    `gorm:"type:uuid;not null;index"`
		UserID           string    `gorm:"type:uuid;not null;index"`
		Rating           int       `gorm:"not null;check:rating >= 1 AND rating <= 5"`
		CreatedAt        time.Time
		UpdatedAt        time.Time
	}

	type FlashcardDeck struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID   string    `gorm:"type:uuid;not null;index"`
		CreatedBy   string    `gorm:"type:uuid;not null;index"`
		Title       string    `gorm:"size:200;not null"`
		Description string    `gorm:"type:text"`
		IsPublic    bool      `gorm:"default:false"`
		CreatedAt   time.Time `gorm:"index"`
		UpdatedAt   time.Time
	}

	type Flashcard struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		DeckID     string    `gorm:"type:uuid;not null;index"`
		FrontText  string    `gorm:"type:text;not null"`
		BackText   string    `gorm:"type:text;not null"`
		OrderIndex int       `gorm:"not null;default:0;index"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type UserFlashcardProgress struct {
		ID             string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		UserID         string     `gorm:"type:uuid;not null;index"`
		FlashcardID    string     `gorm:"type:uuid;not null;index"`
		DeckID         string     `gorm:"type:uuid;not null;index"`
		EaseFactor     float64    `gorm:"default:2.5"`
		Interval       int        `gorm:"default:0"`
		Repetitions    int        `gorm:"default:0"`
		NextReviewDate *time.Time `gorm:"index"`
		LastReviewedAt *time.Time
		TotalReviews   int        `gorm:"default:0"`
		CorrectReviews int        `gorm:"default:0"`
		CreatedAt      time.Time
		UpdatedAt      time.Time
	}

	type FlashcardStudySession struct {
		ID              string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		UserID          string     `gorm:"type:uuid;not null;index"`
		DeckID          string     `gorm:"type:uuid;not null;index"`
		CardsStudied    int        `gorm:"default:0"`
		CardsCorrect    int        `gorm:"default:0"`
		DurationSeconds int        `gorm:"default:0"`
		StartedAt       time.Time  `gorm:"index"`
		CompletedAt     *time.Time
		CreatedAt       time.Time
	}

	type FlashcardReview struct {
		ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SessionID   string    `gorm:"type:uuid;not null;index"`
		FlashcardID string    `gorm:"type:uuid;not null;index"`
		UserID      string    `gorm:"type:uuid;not null;index"`
		Quality     int       `gorm:"not null;check:quality >= 0 AND quality <= 5"`
		CreatedAt   time.Time `gorm:"index"`
	}

	// Drop English language columns if they exist
	// This is a one-time migration to remove English fields from the database
	if err := dropEnglishColumns(db); err != nil {
		log.Printf("Warning: Failed to drop English columns: %v", err)
		// Continue anyway as columns might already be dropped
	}

	// Auto-migrate all models
	err := db.AutoMigrate(&User{}, &Semester{}, &Subject{}, &SubjectTeacher{}, &DocumentCategory{}, &Document{}, &Activity{}, &Comment{}, &Question{}, &Answer{}, &TeacherRating{}, &FlashcardDeck{}, &Flashcard{}, &UserFlashcardProgress{}, &FlashcardStudySession{}, &FlashcardReview{})
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Add unique constraint for teacher ratings (one rating per user per teacher)
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS unique_user_teacher_rating
		ON teacher_ratings(subject_teacher_id, user_id)
	`).Error; err != nil {
		log.Printf("Warning: Failed to create unique constraint on teacher_ratings: %v", err)
		// Continue anyway as constraint might already exist
	}

	// Add unique constraint for user flashcard progress (one progress record per user per flashcard)
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS unique_user_flashcard_progress
		ON user_flashcard_progress(user_id, flashcard_id)
	`).Error; err != nil {
		log.Printf("Warning: Failed to create unique constraint on user_flashcard_progress: %v", err)
	}

	// Add unique constraint for flashcard order within deck
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS unique_deck_card_order
		ON flashcards(deck_id, order_index)
	`).Error; err != nil {
		log.Printf("Warning: Failed to create unique constraint on flashcards: %v", err)
	}

	// Add composite index for deck queries
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_deck_subject_creator
		ON flashcard_decks(subject_id, created_by)
	`).Error; err != nil {
		log.Printf("Warning: Failed to create composite index on flashcard_decks: %v", err)
	}

	// Add many-to-many table for deck favorites
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_favorite_decks (
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			flashcard_deck_id UUID REFERENCES flashcard_decks(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, flashcard_deck_id)
		)
	`).Error; err != nil {
		log.Printf("Warning: Failed to create user_favorite_decks table: %v", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

// dropEnglishColumns drops English language columns from tables
// This is a one-time migration to remove bilingual support
func dropEnglishColumns(db *gorm.DB) error {
	log.Println("Dropping English language columns...")

	// List of columns to drop
	migrations := []struct {
		table  string
		column string
	}{
		{"semesters", "name_en"},
		{"subjects", "name_en"},
		{"subjects", "description_en"},
		{"subject_teachers", "topic_en"},
	}

	for _, m := range migrations {
		// Check if column exists before attempting to drop
		var columnExists bool
		query := `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = ?
				AND column_name = ?
			)
		`
		if err := db.Raw(query, m.table, m.column).Scan(&columnExists).Error; err != nil {
			log.Printf("Warning: Failed to check if column %s.%s exists: %v", m.table, m.column, err)
			continue
		}

		if columnExists {
			sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", m.table, m.column)
			if err := db.Exec(sql).Error; err != nil {
				log.Printf("Warning: Failed to drop column %s.%s: %v", m.table, m.column, err)
				// Continue with other columns even if one fails
				continue
			}
			log.Printf("Dropped column %s.%s", m.table, m.column)
		} else {
			log.Printf("Column %s.%s does not exist, skipping", m.table, m.column)
		}
	}

	log.Println("English language columns dropped successfully")
	return nil
}

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

	type DocumentType struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID  string    `gorm:"type:uuid;not null;index:idx_doctype_subject"`
		NameCS     string    `gorm:"size:100;not null"`
		Icon       string    `gorm:"size:50;default:'folder'"`
		OrderIndex int       `gorm:"not null;default:0;index:idx_doctype_order"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type DocumentCategory struct {
		ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID  string    `gorm:"type:uuid;not null;index:idx_category_subject"`
		TypeID     *string   `gorm:"type:uuid;index:idx_category_type;default:null"` // Nullable for migration
		NameCS     string    `gorm:"size:200;not null"`
		OrderIndex int       `gorm:"not null;default:0;index:idx_category_order"`
		CreatedBy  string    `gorm:"type:uuid;not null"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	type Document struct {
		ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
		SubjectID    string    `gorm:"type:uuid;not null;index"`
		UploadedBy   string    `gorm:"type:uuid;not null;index"`
		AnswerID     *string   `gorm:"type:uuid;index"`
		TypeID       *string   `gorm:"type:uuid;index;default:null"` // Nullable for migration
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

	// Ensure flashcard columns exist (GORM AutoMigrate may miss these on existing tables)
	db.Exec(`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS card_type VARCHAR(20) DEFAULT 'standard'`)
	db.Exec(`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS options TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS correct_option INTEGER DEFAULT -1`)

	// Ensure user_flashcard_progress table exists (GORM AutoMigrate may fail to create it)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_flashcard_progress (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id),
		flashcard_id UUID NOT NULL REFERENCES flashcards(id),
		deck_id UUID NOT NULL REFERENCES flashcard_decks(id),
		ease_factor DOUBLE PRECISION DEFAULT 2.5,
		interval INTEGER DEFAULT 0,
		repetitions INTEGER DEFAULT 0,
		next_review_date TIMESTAMPTZ,
		last_reviewed_at TIMESTAMPTZ,
		total_reviews INTEGER DEFAULT 0,
		correct_reviews INTEGER DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_flashcard_progress_user ON user_flashcard_progress(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_flashcard_progress_flashcard ON user_flashcard_progress(flashcard_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_flashcard_progress_deck ON user_flashcard_progress(deck_id)`)

	// Auto-migrate all models first (this creates the type_id columns)
	err := db.AutoMigrate(&User{}, &Semester{}, &Subject{}, &SubjectTeacher{}, &DocumentType{}, &DocumentCategory{}, &Document{}, &Activity{}, &Comment{}, &Question{}, &Answer{}, &TeacherRating{}, &FlashcardDeck{}, &Flashcard{}, &UserFlashcardProgress{}, &FlashcardStudySession{}, &FlashcardReview{})
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Migrate from old type string to new type_id foreign key (after columns exist)
	if err := migrateDocumentTypes(db); err != nil {
		log.Printf("Warning: Failed to migrate document types: %v", err)
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
		{"document_types", "name_en"},
		{"document_categories", "name_en"},
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

// migrateDocumentTypes migrates from old type string to new document_types table
func migrateDocumentTypes(db *gorm.DB) error {
	log.Println("Checking document types migration...")

	// Check if old 'type' column exists on document_categories
	var typeColumnExists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'document_categories'
			AND column_name = 'type'
		)
	`
	if err := db.Raw(query).Scan(&typeColumnExists).Error; err != nil {
		return fmt.Errorf("failed to check if type column exists: %w", err)
	}

	if !typeColumnExists {
		log.Println("Document types already migrated, skipping")
		return nil
	}

	log.Println("Migrating document types...")

	// Get all unique subject IDs
	var subjectIDs []string
	if err := db.Raw("SELECT DISTINCT id FROM subjects").Scan(&subjectIDs).Error; err != nil {
		return fmt.Errorf("failed to get subjects: %w", err)
	}

	// Default types mapping
	defaultTypes := []struct {
		OldType string
		NameCS  string
		Icon    string
		Order   int
	}{
		{"lecture", "Přednášky", "book-open", 0},
		{"seminar", "Cvičení", "users", 1},
		{"exam", "Testy/Klauzury", "file-text", 2},
		{"other", "Ostatní", "folder", 3},
	}

	for _, subjectID := range subjectIDs {
		// Create document types for this subject and update references in one transaction
		for _, dt := range defaultTypes {
			// Insert new document type and get its ID
			var newTypeID string
			if err := db.Raw(`
				INSERT INTO document_types (id, subject_id, name_cs, icon, order_index, created_at, updated_at)
				VALUES (gen_random_uuid(), ?, ?, ?, ?, NOW(), NOW())
				RETURNING id
			`, subjectID, dt.NameCS, dt.Icon, dt.Order).Scan(&newTypeID).Error; err != nil {
				log.Printf("Warning: Failed to create document type for subject %s: %v", subjectID, err)
				continue
			}

			// Update document_categories to use type_id
			if err := db.Exec(`
				UPDATE document_categories
				SET type_id = ?
				WHERE subject_id = ? AND type = ?
			`, newTypeID, subjectID, dt.OldType).Error; err != nil {
				log.Printf("Warning: Failed to update categories for subject %s, type %s: %v", subjectID, dt.OldType, err)
			}

			// Update documents to use type_id
			if err := db.Exec(`
				UPDATE documents
				SET type_id = ?
				WHERE subject_id = ? AND type = ?
			`, newTypeID, subjectID, dt.OldType).Error; err != nil {
				log.Printf("Warning: Failed to update documents for subject %s, type %s: %v", subjectID, dt.OldType, err)
			}
		}
	}

	// Drop old type columns
	log.Println("Dropping old type columns...")
	if err := db.Exec("ALTER TABLE document_categories DROP COLUMN IF EXISTS type").Error; err != nil {
		log.Printf("Warning: Failed to drop type column from document_categories: %v", err)
	}
	if err := db.Exec("ALTER TABLE documents DROP COLUMN IF EXISTS type").Error; err != nil {
		log.Printf("Warning: Failed to drop type column from documents: %v", err)
	}

	// Add NOT NULL constraint to type_id columns after migration
	log.Println("Adding NOT NULL constraints to type_id columns...")
	if err := db.Exec("ALTER TABLE document_categories ALTER COLUMN type_id SET NOT NULL").Error; err != nil {
		log.Printf("Warning: Failed to add NOT NULL to document_categories.type_id: %v", err)
	}
	if err := db.Exec("ALTER TABLE documents ALTER COLUMN type_id SET NOT NULL").Error; err != nil {
		log.Printf("Warning: Failed to add NOT NULL to documents.type_id: %v", err)
	}

	log.Println("Document types migration completed")
	return nil
}


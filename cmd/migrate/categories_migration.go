package main

import (
	"fmt"
	"log"
	"os"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/database"
	"github.com/P3chys/entoo2-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	log.Println("Starting document categories migration...")

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migration
	if err := MigrateDocumentCategories(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
		os.Exit(1)
	}

	log.Println("Migration completed successfully!")
}

func MigrateDocumentCategories(db *gorm.DB) error {
	log.Println("Step 1: Checking if category column needs to be renamed...")

	// Check if 'category' column exists and rename to 'type' if needed
	var categoryColumnExists bool
	err := db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='documents' AND column_name='category')").Scan(&categoryColumnExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for category column: %w", err)
	}

	if categoryColumnExists {
		log.Println("Renaming 'category' column to 'type'...")
		err = db.Exec("ALTER TABLE documents RENAME COLUMN category TO type").Error
		if err != nil {
			return fmt.Errorf("failed to rename category column: %w", err)
		}
		log.Println("Column renamed successfully")
	} else {
		log.Println("Column already renamed or doesn't exist")
	}

	log.Println("Step 2: Creating 'Unassigned' categories for all subjects...")

	// Get all subjects
	var subjects []models.Subject
	if err := db.Find(&subjects).Error; err != nil {
		return fmt.Errorf("failed to fetch subjects: %w", err)
	}

	log.Printf("Found %d subjects", len(subjects))

	// Get first admin user to use as creator
	var adminUser models.User
	if err := db.Where("role = ?", models.RoleAdmin).First(&adminUser).Error; err != nil {
		log.Println("Warning: No admin user found, using system UUID")
		adminUser.ID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
	}

	types := []string{"lecture", "seminar", "other"}
	categoriesCreated := 0

	for _, subject := range subjects {
		for idx, docType := range types {
			// Check if category already exists
			var existingCategory models.DocumentCategory
			err := db.Where("subject_id = ? AND type = ? AND name_cs = ?",
				subject.ID, docType, "Nepřiřazeno").First(&existingCategory).Error

			if err == gorm.ErrRecordNotFound {
				// Create new "Unassigned" category
				category := models.DocumentCategory{
					SubjectID:  subject.ID,
					Type:       docType,
					NameCS:     "Nepřiřazeno",
					NameEN:     "Unassigned",
					OrderIndex: 999, // Put at end
					CreatedBy:  adminUser.ID,
				}

				if err := db.Create(&category).Error; err != nil {
					return fmt.Errorf("failed to create category for subject %s, type %s: %w",
						subject.ID, docType, err)
				}
				categoriesCreated++
			} else if err != nil {
				return fmt.Errorf("failed to check existing category: %w", err)
			}

			// For backwards compatibility: ensure we have at least OrderIndex set correctly
			if err == nil && existingCategory.OrderIndex != 999 {
				db.Model(&existingCategory).Update("order_index", 999)
			}

			_ = idx // unused
		}
	}

	log.Printf("Created %d new 'Unassigned' categories", categoriesCreated)

	log.Println("Step 3: Linking existing documents to 'Unassigned' categories...")

	// Update documents that don't have a category_id set
	result := db.Exec(`
		UPDATE documents d
		SET category_id = dc.id
		FROM document_categories dc
		WHERE d.subject_id = dc.subject_id
		  AND d.type = dc.type
		  AND dc.name_cs = 'Nepřiřazeno'
		  AND d.category_id IS NULL
	`)

	if result.Error != nil {
		return fmt.Errorf("failed to link documents to categories: %w", result.Error)
	}

	log.Printf("Linked %d documents to 'Unassigned' categories", result.RowsAffected)

	log.Println("Step 4: Verifying migration...")

	// Check for orphaned documents
	var orphanedCount int64
	db.Model(&models.Document{}).Where("category_id IS NULL").Count(&orphanedCount)

	if orphanedCount > 0 {
		log.Printf("Warning: Found %d documents without a category_id", orphanedCount)
	} else {
		log.Println("All documents have been assigned to categories")
	}

	return nil
}

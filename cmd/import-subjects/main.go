package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/database"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Get or create "Unassigned" semester
	semesterID, err := getOrCreateUnassignedSemester(db)
	if err != nil {
		log.Fatalf("Failed to get/create unassigned semester: %v", err)
	}

	log.Printf("Using semester ID: %s", semesterID)

	// Get or create admin user for file uploads
	adminID, err := getOrCreateAdminUser(db)
	if err != nil {
		log.Fatalf("Failed to get/create admin user: %v", err)
	}

	log.Printf("Using admin user ID: %s", adminID)

	// Initialize storage service
	storageService, err := services.NewStorageService(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}

	// Initialize search service
	searchService := services.NewSearchService(cfg)
	log.Println("Meilisearch service initialized")

	// Import subjects from directory
	// Try Linux path first (for Docker), then Windows path
	subjectsDir := `/old_entoo/entoo_subjects`
	if _, err := os.Stat(subjectsDir); os.IsNotExist(err) {
		subjectsDir = `E:\old_entoo\entoo_subjects`
		if _, err := os.Stat(subjectsDir); os.IsNotExist(err) {
			log.Fatalf("Subjects directory does not exist: %s", subjectsDir)
		}
	}

	importedSubjects, err := importSubjects(db, storageService, subjectsDir, semesterID, adminID)
	if err != nil {
		log.Fatalf("Failed to import subjects: %v", err)
	}

	// Index all imported subjects in Meilisearch
	if len(importedSubjects) > 0 {
		log.Printf("Indexing %d subjects in Meilisearch...", len(importedSubjects))
		if err := searchService.IndexSubjects(importedSubjects); err != nil {
			log.Printf("Warning: Failed to index subjects in Meilisearch: %v", err)
		} else {
			log.Printf("Successfully indexed %d subjects in Meilisearch", len(importedSubjects))
		}
	}

	log.Println("Import completed successfully!")
}

func getOrCreateUnassignedSemester(db *gorm.DB) (uuid.UUID, error) {
	var semester models.Semester

	// Try to find existing "Unassigned" semester
	err := db.Where("name_cs = ? OR name_en = ?", "Nepřiřazeno", "Unassigned").First(&semester).Error

	if err == nil {
		log.Printf("Found existing unassigned semester: %s", semester.ID)
		return semester.ID, nil
	}

	if err != gorm.ErrRecordNotFound {
		return uuid.Nil, fmt.Errorf("error checking for semester: %w", err)
	}

	// Create new unassigned semester
	semester = models.Semester{
		ID:         uuid.New(),
		NameCS:     "Nepřiřazeno",
		OrderIndex: 999, // Put at the end
	}

	if err := db.Create(&semester).Error; err != nil {
		return uuid.Nil, fmt.Errorf("failed to create semester: %w", err)
	}

	log.Printf("Created new unassigned semester: %s", semester.ID)
	return semester.ID, nil
}

func getOrCreateAdminUser(db *gorm.DB) (uuid.UUID, error) {
	var user models.User

	// Try to find an admin user
	err := db.Where("role = ?", "admin").First(&user).Error

	if err == nil {
		log.Printf("Found existing admin user: %s (%s)", user.Email, user.ID)
		return user.ID, nil
	}

	if err != gorm.ErrRecordNotFound {
		return uuid.Nil, fmt.Errorf("error checking for admin user: %w", err)
	}

	// If no admin exists, find any user
	err = db.First(&user).Error
	if err == nil {
		log.Printf("Using existing user: %s (%s)", user.Email, user.ID)
		return user.ID, nil
	}

	return uuid.Nil, fmt.Errorf("no users found in database, please create an admin user first")
}

func importSubjects(db *gorm.DB, storage *services.StorageService, baseDir string, semesterID uuid.UUID, uploaderID uuid.UUID) ([]models.Subject, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	imported := 0
	skipped := 0
	errors := 0
	totalFiles := 0
	uploadedFiles := 0
	var importedSubjects []models.Subject

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subjectName := entry.Name()

		// Skip test directories
		if strings.ToLower(subjectName) == "test" {
			skipped++
			continue
		}

		// Check if subject already exists
		var existing models.Subject
		err := db.Where("name_cs = ?", subjectName).First(&existing).Error
		if err == nil {
			log.Printf("Subject already exists, skipping: %s", subjectName)
			skipped++
			continue
		}
		if err != gorm.ErrRecordNotFound {
			log.Printf("Error checking subject %s: %v", subjectName, err)
			errors++
			continue
		}

		// Generate a code from the subject name
		code := generateSubjectCode(subjectName)

		// Ensure code is unique
		originalCode := code
		counter := 1
		for {
			var codeCheck models.Subject
			err := db.Where("code = ?", code).First(&codeCheck).Error
			if err == gorm.ErrRecordNotFound {
				break
			}
			code = fmt.Sprintf("%s%d", originalCode, counter)
			counter++
			if len(code) > 6 {
				code = code[:6]
			}
		}

		// Create the subject
		subject := models.Subject{
			ID:            uuid.New(),
			SemesterID:    semesterID,
			NameCS:        subjectName,
			Code:          code,
			DescriptionCS: "",
			Credits:       0,
		}

		if err := db.Create(&subject).Error; err != nil {
			log.Printf("Failed to create subject %s: %v", subjectName, err)
			errors++
			continue
		}

		log.Printf("Imported subject: %s (code: %s, id: %s)", subjectName, code, subject.ID)
		imported++
		importedSubjects = append(importedSubjects, subject)

		// Upload files from subject directory
		subjectDir := filepath.Join(baseDir, subjectName)
		fileCount, uploadCount := uploadSubjectFiles(db, storage, subjectDir, subject.ID, uploaderID)
		totalFiles += fileCount
		uploadedFiles += uploadCount

		if fileCount > 0 {
			log.Printf("  -> Uploaded %d/%d files", uploadCount, fileCount)
		}
	}

	log.Printf("\nImport Summary:")
	log.Printf("  Subjects imported: %d", imported)
	log.Printf("  Subjects skipped: %d", skipped)
	log.Printf("  Errors: %d", errors)
	log.Printf("  Files uploaded: %d/%d", uploadedFiles, totalFiles)

	return importedSubjects, nil
}

func generateSubjectCode(name string) string {
	// Clean the name
	cleaned := strings.TrimSpace(name)

	// Take first letters or generate a simple code
	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return "SUBJ"
	}

	var code strings.Builder
	for i, word := range words {
		if i >= 3 { // Max 3 letters
			break
		}
		if len(word) > 0 {
			// Take first letter of each word
			code.WriteRune([]rune(strings.ToUpper(word))[0])
		}
	}

	result := code.String()
	if len(result) == 0 {
		return "SUBJ"
	}

	// Ensure it's between 3-6 characters
	if len(result) < 3 {
		result = result + strings.Repeat("X", 3-len(result))
	}
	if len(result) > 6 {
		result = result[:6]
	}

	return result
}

func uploadSubjectFiles(db *gorm.DB, storage *services.StorageService, subjectDir string, subjectID uuid.UUID, uploaderID uuid.UUID) (int, int) {
	totalFiles := 0
	uploadedFiles := 0

	err := filepath.WalkDir(subjectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if d.IsDir() {
			return nil // Continue walking
		}

		totalFiles++

		// Open the file
		file, err := os.Open(path)
		if err != nil {
			log.Printf("    Failed to open file %s: %v", path, err)
			return nil
		}
		defer file.Close()

		// Get file info
		fileInfo, err := file.Stat()
		if err != nil {
			log.Printf("    Failed to stat file %s: %v", path, err)
			return nil
		}

		// Get relative path from subject directory
		relPath, err := filepath.Rel(subjectDir, path)
		if err != nil {
			relPath = filepath.Base(path)
		}

		// Generate unique filename for MinIO
		ext := filepath.Ext(path)
		filename := fmt.Sprintf("%s/%s%s", subjectID.String(), uuid.New().String(), ext)

		// Detect MIME type
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		// Upload to MinIO
		ctx := context.Background()
		_, err = storage.UploadFileFromPath(ctx, file, filename, fileInfo.Size(), mimeType)
		if err != nil {
			log.Printf("    Failed to upload file %s: %v", relPath, err)
			return nil
		}

		// Create document record
		document := models.Document{
			ID:           uuid.New(),
			SubjectID:    subjectID,
			UploadedBy:   uploaderID,
			Filename:     filename,
			OriginalName: relPath,
			FileSize:     fileInfo.Size(),
			MimeType:     mimeType,
			MinIOPath:    filename,
		}

		if err := db.Create(&document).Error; err != nil {
			log.Printf("    Failed to create document record for %s: %v", relPath, err)
			// Try to delete the uploaded file from MinIO
			_ = storage.DeleteFile(filename)
			return nil
		}

		uploadedFiles++
		return nil
	})

	if err != nil {
		log.Printf("  Error walking directory: %v", err)
	}

	return totalFiles, uploadedFiles
}

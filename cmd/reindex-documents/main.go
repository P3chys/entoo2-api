package main

import (
	"log"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/database"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/joho/godotenv"
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

	// Initialize search service
	searchService := services.NewSearchService(cfg)
	log.Println("Meilisearch service initialized")

	// Get counts
	var dbCount int64
	if err := db.Model(&models.Document{}).Count(&dbCount).Error; err != nil {
		log.Fatalf("Failed to get document count from DB: %v", err)
	}

	meiliCount, err := searchService.GetDocumentCount()
	if err != nil {
		log.Fatalf("Failed to get document count from Meilisearch: %v", err)
	}

	log.Printf("Documents in DB: %d", dbCount)
	log.Printf("Documents in Meilisearch: %d", meiliCount)

	if meiliCount == dbCount {
		log.Println("Counts match. Verifying all documents are indexed...")
	} else {
		log.Println("Counts do not match. Reindexing all documents...")
	}

	// Fetch all documents in batches
	batchSize := 100
	var offset int
	totalIndexed := 0

	for {
		var documents []models.Document
		if err := db.Limit(batchSize).Offset(offset).Find(&documents).Error; err != nil {
			log.Fatalf("Failed to fetch documents: %v", err)
		}

		if len(documents) == 0 {
			break
		}

		if err := searchService.IndexDocuments(documents); err != nil {
			log.Printf("Failed to index batch (offset %d): %v", offset, err)
		} else {
			totalIndexed += len(documents)
			log.Printf("Indexed batch of %d documents (total: %d)", len(documents), totalIndexed)
		}

		offset += batchSize
		time.Sleep(100 * time.Millisecond) // Be nice to Meilisearch
	}

	// Final check
	finalMeiliCount, err := searchService.GetDocumentCount()
	if err != nil {
		log.Printf("Failed to get final count: %v", err)
	}

	log.Printf("Reindexing completed.")
	log.Printf("Final Meilisearch count: %d", finalMeiliCount)
}

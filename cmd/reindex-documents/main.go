// reindex-documents is a maintenance utility.
// With the pg-only stack, full-text search is handled by a GENERATED tsvector column
// (content_tsv) on the documents table, so no external search engine needs reindexing.
//
// This command now performs VACUUM ANALYZE on the documents table to ensure
// FTS statistics are up-to-date after bulk operations.
package main

import (
	"log"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/database"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	log.Println("Running VACUUM ANALYZE on documents table to refresh FTS statistics...")
	if err := db.Exec("VACUUM ANALYZE documents").Error; err != nil {
		log.Fatalf("VACUUM ANALYZE failed: %v", err)
	}

	var count int64
	db.Raw("SELECT COUNT(*) FROM documents").Scan(&count)
	log.Printf("Done. Documents in database: %d", count)
}

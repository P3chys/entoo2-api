package database

import (
	"log"
	"os"

	"github.com/P3chys/entoo2-api/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAdmin creates a default admin account if no admin exists in the database
func SeedAdmin(db *gorm.DB) error {
	// Check if any admin user exists
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("Admin user already exists, skipping seed")
		return nil
	}

	// Get admin credentials from environment or use defaults
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@entoo2.local"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "AdminPassword123!"
	}

	adminName := os.Getenv("ADMIN_NAME")
	if adminName == "" {
		adminName = "System Administrator"
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Create admin user
	admin := models.User{
		Email:        adminEmail,
		PasswordHash: string(hashedPassword),
		Role:         models.RoleAdmin,
		DisplayName:  adminName,
		Language:     "en",
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Printf("Created default admin user: %s", adminEmail)
	return nil
}

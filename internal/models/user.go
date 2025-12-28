package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleStudent UserRole = "student"
	RoleAdmin   UserRole = "admin"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         UserRole  `gorm:"type:varchar(20);default:'student'" json:"role"`
	DisplayName  string    `gorm:"size:100" json:"display_name"`
	Language     string    `gorm:"type:varchar(2);default:'cs'" json:"language"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Email Verification
	EmailVerified           bool       `gorm:"default:false" json:"email_verified"`
	EmailVerificationToken  *string    `gorm:"size:255;index" json:"-"`
	EmailVerificationSentAt *time.Time `json:"-"`
	EmailVerifiedAt         *time.Time `json:"email_verified_at,omitempty"`

	// Password Reset
	PasswordResetToken     *string    `gorm:"size:255;index" json:"-"`
	PasswordResetSentAt    *time.Time `json:"-"`
	PasswordResetExpiresAt *time.Time `json:"-"`

	// Favorites
	FavoriteSubjects  []Subject  `gorm:"many2many:user_favorite_subjects;" json:"favorite_subjects,omitempty"`
	FavoriteDocuments []Document `gorm:"many2many:user_favorite_documents;" json:"favorite_documents,omitempty"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

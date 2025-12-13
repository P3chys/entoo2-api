package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActivityType string

const (
	ActivityDocumentUploaded ActivityType = "document_uploaded"
	ActivityDocumentDeleted  ActivityType = "document_deleted"
)

type Activity struct {
	ID           uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID    `gorm:"type:uuid;not null;index" json:"user_id"`
	ActivityType ActivityType `gorm:"type:varchar(50);not null;index" json:"activity_type"`
	SubjectID    *uuid.UUID   `gorm:"type:uuid;index" json:"subject_id,omitempty"`
	DocumentID   *uuid.UUID   `gorm:"type:uuid;index" json:"document_id,omitempty"`
	Metadata     string       `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`

	// Relations
	User     User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Subject  *Subject  `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
}

func (Activity) TableName() string {
	return "activities"
}

func (a *Activity) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	return nil
}

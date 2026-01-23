package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentType represents a custom document type/tab for a subject
// Each subject can have its own set of types (e.g., "Přednášky", "Cvičení", "Skripta")
type DocumentType struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID  uuid.UUID `gorm:"type:uuid;not null;index:idx_doctype_subject" json:"subject_id"`
	NameCS     string    `gorm:"size:100;not null" json:"name_cs"`
	Icon       string    `gorm:"size:50;default:'folder'" json:"icon"` // Icon name for UI
	OrderIndex int       `gorm:"not null;default:0;index:idx_doctype_order" json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relations
	Subject    Subject            `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Categories []DocumentCategory `gorm:"foreignKey:TypeID" json:"categories,omitempty"`
}

func (DocumentType) TableName() string {
	return "document_types"
}

func (dt *DocumentType) BeforeCreate(tx *gorm.DB) error {
	if dt.ID == uuid.Nil {
		dt.ID = uuid.New()
	}
	return nil
}

// DefaultDocumentTypes returns the default types to create for a new subject
func DefaultDocumentTypes() []struct {
	NameCS string
	Icon   string
	Order  int
} {
	return []struct {
		NameCS string
		Icon   string
		Order  int
	}{
		{NameCS: "Přednášky", Icon: "book-open", Order: 0},
		{NameCS: "Cvičení", Icon: "users", Order: 1},
		{NameCS: "Testy/Klauzury", Icon: "file-text", Order: 2},
		{NameCS: "Ostatní", Icon: "folder", Order: 3},
	}
}

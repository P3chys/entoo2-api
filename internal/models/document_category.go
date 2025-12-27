package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentCategory struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID  uuid.UUID `gorm:"type:uuid;not null;index:idx_subject_type" json:"subject_id"`
	Type       string    `gorm:"size:20;not null;index:idx_subject_type" json:"type"` // lecture, seminar, other
	NameCS     string    `gorm:"size:200;not null" json:"name_cs"`
	NameEN     string    `gorm:"size:200;not null" json:"name_en"`
	OrderIndex int       `gorm:"not null;default:0;index:idx_order" json:"order_index"`
	CreatedBy  uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relations
	Subject   Subject    `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Documents []Document `gorm:"foreignKey:CategoryID" json:"documents,omitempty"`
}

func (DocumentCategory) TableName() string {
	return "document_categories"
}

func (dc *DocumentCategory) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == uuid.Nil {
		dc.ID = uuid.New()
	}
	return nil
}

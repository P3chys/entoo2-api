package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Document struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID    uuid.UUID `gorm:"type:uuid;not null;index" json:"subject_id"`
	UploadedBy   uuid.UUID `gorm:"type:uuid;not null;index" json:"uploaded_by"`
	AnswerID     *uuid.UUID `gorm:"type:uuid;index" json:"answer_id,omitempty"` // Link to answer if attached
	Filename     string    `gorm:"size:255;not null" json:"filename"`
	OriginalName string    `gorm:"size:255;not null" json:"original_name"`
	FileSize     int64     `gorm:"not null" json:"file_size"`
	MimeType     string    `gorm:"size:100;not null" json:"mime_type"`
	MinIOPath    string    `gorm:"size:500;not null" json:"minio_path"`
	ContentText  string    `gorm:"type:text" json:"content_text,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`

	// Relations
	Subject  Subject `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	Uploader User    `gorm:"foreignKey:UploadedBy" json:"uploader,omitempty"`

	// Computed
	IsFavorite bool `gorm:"->" json:"is_favorite"`
}

func (Document) TableName() string {
	return "documents"
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

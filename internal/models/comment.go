package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Comment struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID   uuid.UUID `gorm:"type:uuid;not null" json:"subject_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	IsAnonymous bool      `gorm:"default:false" json:"is_anonymous"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Comment) TableName() string {
	return "comments"
}

func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

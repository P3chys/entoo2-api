package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Semester struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NameCS     string    `gorm:"size:100;not null" json:"name_cs"`
	NameEN     string    `gorm:"size:100;not null" json:"name_en"`
	OrderIndex int       `gorm:"default:0" json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relations
	Subjects []Subject `gorm:"foreignKey:SemesterID" json:"subjects,omitempty"`
}

func (Semester) TableName() string {
	return "semesters"
}

func (s *Semester) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

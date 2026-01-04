package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TeacherRating struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectTeacherID uuid.UUID `gorm:"type:uuid;not null;index" json:"subject_teacher_id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Rating           int       `gorm:"not null;check:rating >= 1 AND rating <= 5" json:"rating"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	SubjectTeacher SubjectTeacher `gorm:"foreignKey:SubjectTeacherID" json:"subject_teacher,omitempty"`
	User           User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (TeacherRating) TableName() string {
	return "teacher_ratings"
}

func (tr *TeacherRating) BeforeCreate(tx *gorm.DB) error {
	if tr.ID == uuid.Nil {
		tr.ID = uuid.New()
	}
	return nil
}

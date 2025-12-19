package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Question struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID   uuid.UUID `gorm:"type:uuid;not null;index" json:"subject_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	IsAnonymous bool      `gorm:"default:false" json:"is_anonymous"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User    User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Subject Subject  `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	Answers []Answer `gorm:"foreignKey:QuestionID;constraint:OnDelete:CASCADE;" json:"answers,omitempty"`
}

func (Question) TableName() string {
	return "questions"
}

func (q *Question) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

type Answer struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	QuestionID uuid.UUID  `gorm:"type:uuid;not null;index" json:"question_id"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Content    string     `gorm:"type:text;not null" json:"content"`
	DocumentID *uuid.UUID `gorm:"type:uuid;index" json:"document_id,omitempty"` // Optional attachment
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	// Relations
	User     User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Question Question  `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
}

func (Answer) TableName() string {
	return "answers"
}

func (a *Answer) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

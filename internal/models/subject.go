package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Subject struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SemesterID    uuid.UUID `gorm:"type:uuid;not null" json:"semester_id"`
	NameCS        string    `gorm:"size:200;not null" json:"name_cs"`
	Code          string    `gorm:"size:10;uniqueIndex" json:"code"`
	DescriptionCS string    `gorm:"type:text" json:"description_cs"`
	Credits       int       `gorm:"default:0" json:"credits"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relations
	Semester Semester        `gorm:"foreignKey:SemesterID" json:"semester,omitempty"`
	Teachers []SubjectTeacher `gorm:"foreignKey:SubjectID" json:"teachers,omitempty"`
	Documents []Document      `gorm:"foreignKey:SubjectID" json:"documents,omitempty"`
	Comments  []Comment       `gorm:"foreignKey:SubjectID" json:"comments,omitempty"`
	Questions []Question      `gorm:"foreignKey:SubjectID" json:"questions,omitempty"`

	// Computed
	IsFavorite bool `gorm:"->" json:"is_favorite"`
}

func (Subject) TableName() string {
	return "subjects"
}

func (s *Subject) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type SubjectTeacher struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID   uuid.UUID `gorm:"type:uuid;not null" json:"subject_id"`
	TeacherName string    `gorm:"size:200;not null" json:"teacher_name"`
	TopicCS     string    `gorm:"size:300" json:"topic_cs"`
	CreatedAt   time.Time `json:"created_at"`
}

func (SubjectTeacher) TableName() string {
	return "subject_teachers"
}

func (st *SubjectTeacher) BeforeCreate(tx *gorm.DB) error {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	return nil
}

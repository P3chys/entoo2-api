package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlashcardDeck represents a collection of flashcards for a subject
type FlashcardDeck struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubjectID   uuid.UUID `gorm:"type:uuid;not null;index" json:"subject_id"`
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	IsPublic    bool      `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	Subject    Subject      `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	Creator    User         `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Flashcards []Flashcard  `gorm:"foreignKey:DeckID;constraint:OnDelete:CASCADE;" json:"flashcards,omitempty"`

	// Computed fields
	CardCount  int  `gorm:"->" json:"card_count"`
	IsFavorite bool `gorm:"->" json:"is_favorite"`
}

func (FlashcardDeck) TableName() string {
	return "flashcard_decks"
}

func (d *FlashcardDeck) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// CardType represents the type of flashcard
type CardType string

const (
	CardTypeStandard       CardType = "standard"        // Standard front/back card
	CardTypeMultipleChoice CardType = "multiple_choice" // Multiple choice question
)

// Flashcard represents a single flashcard with front and back content
type Flashcard struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DeckID        uuid.UUID `gorm:"type:uuid;not null;index" json:"deck_id"`
	CardType      CardType  `gorm:"type:varchar(20);default:'standard'" json:"card_type"`
	FrontText     string    `gorm:"type:text;not null" json:"front_text"`
	BackText      string    `gorm:"type:text;not null" json:"back_text"`
	Options       string    `gorm:"type:text" json:"options,omitempty"`         // JSON array of options for multiple choice
	CorrectOption int       `gorm:"default:-1" json:"correct_option,omitempty"` // Index of correct option (0-based), -1 for standard cards
	OrderIndex    int       `gorm:"not null;default:0;index" json:"order_index"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relations
	Deck *FlashcardDeck `gorm:"foreignKey:DeckID" json:"deck,omitempty"`
}

func (Flashcard) TableName() string {
	return "flashcards"
}

func (f *Flashcard) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// UserFlashcardProgress tracks user progress on individual flashcards using SM-2 algorithm
type UserFlashcardProgress struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	FlashcardID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"flashcard_id"`
	DeckID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"deck_id"`
	EaseFactor     float64    `gorm:"default:2.5" json:"ease_factor"`           // SM-2 algorithm
	Interval       int        `gorm:"default:0" json:"interval"`                // Days until next review
	Repetitions    int        `gorm:"default:0" json:"repetitions"`             // Consecutive correct
	NextReviewDate *time.Time `gorm:"index" json:"next_review_date"`            // When to review next
	LastReviewedAt *time.Time `json:"last_reviewed_at"`
	TotalReviews   int        `gorm:"default:0" json:"total_reviews"`
	CorrectReviews int        `gorm:"default:0" json:"correct_reviews"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Relations
	User      User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Flashcard Flashcard     `gorm:"foreignKey:FlashcardID" json:"flashcard,omitempty"`
	Deck      FlashcardDeck `gorm:"foreignKey:DeckID" json:"deck,omitempty"`
}

func (UserFlashcardProgress) TableName() string {
	return "user_flashcard_progress"
}

func (p *UserFlashcardProgress) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// FlashcardStudySession tracks a study session for a deck
type FlashcardStudySession struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	DeckID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"deck_id"`
	CardsStudied    int        `gorm:"default:0" json:"cards_studied"`
	CardsCorrect    int        `gorm:"default:0" json:"cards_correct"`
	DurationSeconds int        `gorm:"default:0" json:"duration_seconds"`
	StartedAt       time.Time  `gorm:"index" json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`

	// Relations
	User User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Deck FlashcardDeck `gorm:"foreignKey:DeckID" json:"deck,omitempty"`
}

func (FlashcardStudySession) TableName() string {
	return "flashcard_study_sessions"
}

func (s *FlashcardStudySession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// FlashcardReview tracks individual flashcard reviews during a study session
type FlashcardReview struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SessionID   uuid.UUID `gorm:"type:uuid;not null;index" json:"session_id"`
	FlashcardID uuid.UUID `gorm:"type:uuid;not null;index" json:"flashcard_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Quality     int       `gorm:"not null;check:quality >= 0 AND quality <= 5" json:"quality"` // SM-2: 0-5
	CreatedAt   time.Time `gorm:"index" json:"created_at"`

	// Relations
	Session   FlashcardStudySession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Flashcard Flashcard             `gorm:"foreignKey:FlashcardID" json:"flashcard,omitempty"`
	User      User                  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (FlashcardReview) TableName() string {
	return "flashcard_reviews"
}

func (r *FlashcardReview) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

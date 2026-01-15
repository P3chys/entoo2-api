package services

import (
	"math"
	"time"

	"github.com/P3chys/entoo2-api/internal/models"
)

// SpacedRepetitionService implements the SM-2 algorithm for spaced repetition
type SpacedRepetitionService struct{}

// NewSpacedRepetitionService creates a new spaced repetition service
func NewSpacedRepetitionService() *SpacedRepetitionService {
	return &SpacedRepetitionService{}
}

// ReviewResult contains the calculated values after a review
type ReviewResult struct {
	EaseFactor     float64
	Interval       int
	Repetitions    int
	NextReviewDate time.Time
}

// CalculateNextReview implements the SM-2 algorithm
// Quality scale:
// 0 = Total blackout, complete failure to recall
// 1 = Incorrect response, but upon seeing the correct answer it felt familiar
// 2 = Incorrect response, but the correct answer seemed easy to recall
// 3 = Correct response with serious difficulty
// 4 = Correct response after some hesitation
// 5 = Perfect response with no hesitation
func (s *SpacedRepetitionService) CalculateNextReview(
	progress *models.UserFlashcardProgress,
	quality int,
) ReviewResult {
	// Clamp quality to valid range
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	// Get current values
	easeFactor := progress.EaseFactor
	interval := progress.Interval
	repetitions := progress.Repetitions

	// Initialize ease factor if not set
	if easeFactor < 1.3 {
		easeFactor = 2.5
	}

	if quality < 3 {
		// Failed - reset interval and repetitions
		interval = 1
		repetitions = 0
	} else {
		// Correct - increment repetitions and calculate new interval
		repetitions++

		if repetitions == 1 {
			interval = 1 // 1 day
		} else if repetitions == 2 {
			interval = 6 // 6 days
		} else {
			// interval = previous_interval * ease_factor
			interval = int(math.Round(float64(interval) * easeFactor))
		}
	}

	// Update ease factor using SM-2 formula:
	// EF' = EF + (0.1 - (5-q) * (0.08 + (5-q) * 0.02))
	q := float64(quality)
	easeFactor = easeFactor + (0.1 - (5-q)*(0.08+(5-q)*0.02))

	// Minimum ease factor is 1.3
	if easeFactor < 1.3 {
		easeFactor = 1.3
	}

	// Maximum interval is 365 days (1 year)
	if interval > 365 {
		interval = 365
	}

	// Calculate next review date
	nextReviewDate := time.Now().AddDate(0, 0, interval)

	return ReviewResult{
		EaseFactor:     easeFactor,
		Interval:       interval,
		Repetitions:    repetitions,
		NextReviewDate: nextReviewDate,
	}
}

// IsCardDue checks if a card is due for review
func (s *SpacedRepetitionService) IsCardDue(progress *models.UserFlashcardProgress) bool {
	if progress == nil || progress.NextReviewDate == nil {
		return true // New card, always due
	}
	return progress.NextReviewDate.Before(time.Now()) || progress.NextReviewDate.Equal(time.Now())
}

// GetCardStatus returns the status of a card based on progress
func (s *SpacedRepetitionService) GetCardStatus(progress *models.UserFlashcardProgress) string {
	if progress == nil {
		return "new"
	}
	if progress.Repetitions >= 3 && progress.Interval >= 21 {
		return "mastered"
	}
	return "learning"
}

// CalculateDeckProgress calculates progress statistics for a deck
func (s *SpacedRepetitionService) CalculateDeckProgress(
	totalCards int,
	progressRecords []models.UserFlashcardProgress,
) DeckProgress {
	progress := DeckProgress{
		TotalCards: totalCards,
	}

	if totalCards == 0 {
		return progress
	}

	cardsSeen := make(map[string]bool)
	var totalEaseFactor float64
	var easeFactorCount int
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, p := range progressRecords {
		cardID := p.FlashcardID.String()
		if cardsSeen[cardID] {
			continue
		}
		cardsSeen[cardID] = true

		// Track ease factor
		if p.EaseFactor > 0 {
			totalEaseFactor += p.EaseFactor
			easeFactorCount++
		}

		// Categorize card
		status := s.GetCardStatus(&p)
		switch status {
		case "mastered":
			progress.MasteredCards++
		case "learning":
			progress.LearningCards++
		}

		// Check if due today
		if p.NextReviewDate != nil && !p.NextReviewDate.After(today.AddDate(0, 0, 1)) {
			progress.CardsDueToday++
		}
	}

	// New cards are those without progress records
	progress.NewCards = totalCards - len(cardsSeen)

	// Calculate average ease factor
	if easeFactorCount > 0 {
		progress.AverageEaseFactor = totalEaseFactor / float64(easeFactorCount)
	} else {
		progress.AverageEaseFactor = 2.5 // Default
	}

	return progress
}

// DeckProgress contains progress statistics for a deck
type DeckProgress struct {
	TotalCards           int     `json:"total_cards"`
	MasteredCards        int     `json:"mastered_cards"`
	LearningCards        int     `json:"learning_cards"`
	NewCards             int     `json:"new_cards"`
	CardsDueToday        int     `json:"cards_due_today"`
	AverageEaseFactor    float64 `json:"average_ease_factor"`
	TotalStudyTimeSeconds int    `json:"total_study_time_seconds"`
}

package services

import (
	"fmt"
	"log"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"gorm.io/gorm"
)

type StatsService struct {
	db *gorm.DB
}

func NewStatsService(db *gorm.DB, _ *config.Config) (*StatsService, error) {
	return &StatsService{db: db}, nil
}

// UserStats contains user-related statistics.
type UserStats struct {
	TotalUsers      int64 `json:"total_users"`
	RegisteredToday int64 `json:"registered_today"`
	ActiveLast24h   int64 `json:"active_last_24h"`
	ActiveLast15min int64 `json:"active_last_15min"`
	VerifiedUsers   int64 `json:"verified_users"`
	AdminCount      int64 `json:"admin_count"`
}

func (s *StatsService) GetUserStats() (*UserStats, error) {
	stats := &UserStats{}

	if err := s.db.Model(&models.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.User{}).Where("created_at >= ?", today).Count(&stats.RegisteredToday).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&models.User{}).Where("email_verified = ?", true).Count(&stats.VerifiedUsers).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&stats.AdminCount).Error; err != nil {
		return nil, err
	}

	// Active users derived from the activities table
	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM activities WHERE created_at >= ?`,
		time.Now().Add(-15*time.Minute)).Scan(&stats.ActiveLast15min)
	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM activities WHERE created_at >= ?`,
		time.Now().Add(-24*time.Hour)).Scan(&stats.ActiveLast24h)

	return stats, nil
}

// APIStats contains API usage statistics.
type APIStats struct {
	TotalRequests  int64                `json:"total_requests"`
	RequestsToday  int64                `json:"requests_today"`
	TopEndpoints   []EndpointStats      `json:"top_endpoints"`
	RequestsByHour map[string]int64     `json:"requests_by_hour"`
}

type EndpointStats struct {
	Endpoint string `json:"endpoint"`
	Count    int64  `json:"count"`
}

func (s *StatsService) GetAPIStats() (*APIStats, error) {
	stats := &APIStats{
		TopEndpoints:   make([]EndpointStats, 0),
		RequestsByHour: make(map[string]int64),
	}

	// Total requests
	s.db.Raw("SELECT COUNT(*) FROM api_requests").Scan(&stats.TotalRequests)

	// Requests today
	s.db.Raw("SELECT COUNT(*) FROM api_requests WHERE created_at >= CURRENT_DATE").Scan(&stats.RequestsToday)

	// Top endpoints (last 30 days)
	type row struct {
		Endpoint string
		Count    int64
	}
	var rows []row
	s.db.Raw(`
		SELECT endpoint, COUNT(*) AS count
		FROM api_requests
		WHERE created_at > NOW() - INTERVAL '30 days'
		GROUP BY endpoint
		ORDER BY count DESC
		LIMIT 10
	`).Scan(&rows)

	for _, r := range rows {
		stats.TopEndpoints = append(stats.TopEndpoints, EndpointStats{Endpoint: r.Endpoint, Count: r.Count})
	}

	// Hourly for today
	type hourRow struct {
		Hour  int
		Count int64
	}
	var hourRows []hourRow
	s.db.Raw(`
		SELECT EXTRACT(HOUR FROM created_at)::int AS hour, COUNT(*) AS count
		FROM api_requests
		WHERE created_at >= CURRENT_DATE
		GROUP BY hour
	`).Scan(&hourRows)

	for h := 0; h < 24; h++ {
		stats.RequestsByHour[fmt.Sprintf("%02d:00", h)] = 0
	}
	for _, r := range hourRows {
		stats.RequestsByHour[fmt.Sprintf("%02d:00", r.Hour)] = r.Count
	}

	return stats, nil
}

// ActivityStats contains activity-related statistics.
type ActivityStats struct {
	TotalActivities  int64             `json:"total_activities"`
	ActivitiesToday  int64             `json:"activities_today"`
	ByType           map[string]int64  `json:"by_type"`
	RecentActivities []models.Activity `json:"recent_activities"`
}

func (s *StatsService) GetActivityStats() (*ActivityStats, error) {
	stats := &ActivityStats{ByType: make(map[string]int64)}

	if err := s.db.Model(&models.Activity{}).Count(&stats.TotalActivities).Error; err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.Activity{}).Where("created_at >= ?", today).Count(&stats.ActivitiesToday).Error; err != nil {
		return nil, err
	}

	type TypeCount struct {
		ActivityType string
		Count        int64
	}
	var typeCounts []TypeCount
	if err := s.db.Model(&models.Activity{}).
		Select("activity_type, count(*) as count").
		Group("activity_type").
		Scan(&typeCounts).Error; err != nil {
		return nil, err
	}
	for _, tc := range typeCounts {
		stats.ByType[tc.ActivityType] = tc.Count
	}

	if err := s.db.Preload("User").Preload("Subject").Preload("Document").
		Order("created_at desc").Limit(20).Find(&stats.RecentActivities).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// SystemStats contains system-wide statistics.
type SystemStats struct {
	TotalDocuments      int64 `json:"total_documents"`
	DocumentsToday      int64 `json:"documents_today"`
	TotalSubjects       int64 `json:"total_subjects"`
	TotalSemesters      int64 `json:"total_semesters"`
	TotalFlashcardDecks int64 `json:"total_flashcard_decks"`
	TotalComments       int64 `json:"total_comments"`
	TotalQuestions      int64 `json:"total_questions"`
}

func (s *StatsService) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.Document{}).Count(&stats.TotalDocuments)
	s.db.Model(&models.Document{}).Where("created_at >= ?", today).Count(&stats.DocumentsToday)
	s.db.Model(&models.Subject{}).Count(&stats.TotalSubjects)
	s.db.Model(&models.Semester{}).Count(&stats.TotalSemesters)
	s.db.Model(&models.FlashcardDeck{}).Count(&stats.TotalFlashcardDecks)
	s.db.Model(&models.Comment{}).Count(&stats.TotalComments)
	s.db.Model(&models.Question{}).Count(&stats.TotalQuestions)

	return stats, nil
}

// DashboardOverview contains all dashboard statistics.
type DashboardOverview struct {
	Users    *UserStats     `json:"users"`
	API      *APIStats      `json:"api"`
	Activity *ActivityStats `json:"activity"`
	System   *SystemStats   `json:"system"`
}

func (s *StatsService) GetDashboardOverview() (*DashboardOverview, error) {
	overview := &DashboardOverview{}

	var err error
	if overview.Users, err = s.GetUserStats(); err != nil {
		return nil, err
	}
	if overview.API, err = s.GetAPIStats(); err != nil {
		return nil, err
	}
	if overview.Activity, err = s.GetActivityStats(); err != nil {
		return nil, err
	}
	if overview.System, err = s.GetSystemStats(); err != nil {
		return nil, err
	}
	return overview, nil
}

// DocumentStats24h contains document upload/download counts for the last 24 hours.
type DocumentStats24h struct {
	Uploaded   int64
	Downloaded int64
}

func (s *StatsService) GetDocumentStats24h() (*DocumentStats24h, error) {
	stats := &DocumentStats24h{}
	cutoff := time.Now().Add(-24 * time.Hour)

	s.db.Model(&models.Activity{}).
		Where("activity_type = ? AND created_at >= ?", models.ActivityDocumentUploaded, cutoff).
		Count(&stats.Uploaded)
	s.db.Model(&models.Activity{}).
		Where("activity_type = ? AND created_at >= ?", models.ActivityDocumentDownloaded, cutoff).
		Count(&stats.Downloaded)

	return stats, nil
}

// StartObservabilityLogger starts a background goroutine that logs stats every 5 minutes.
func (s *StatsService) StartObservabilityLogger() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		s.logStats()
		for range ticker.C {
			s.logStats()
		}
	}()
	log.Println("[STATS] Observability logger started (interval: 5m)")
}

func (s *StatsService) logStats() {
	var totalUsers int64
	if err := s.db.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		log.Printf("[STATS] ERROR: failed to query users: %v", err)
		return
	}

	var activeNow, activeToday int64
	s.db.Raw("SELECT COUNT(DISTINCT user_id) FROM activities WHERE created_at >= ?",
		time.Now().Add(-15*time.Minute)).Scan(&activeNow)
	s.db.Raw("SELECT COUNT(DISTINCT user_id) FROM activities WHERE created_at >= ?",
		time.Now().Add(-24*time.Hour)).Scan(&activeToday)

	docStats, err := s.GetDocumentStats24h()
	if err != nil {
		log.Printf("[STATS] ERROR: failed to query document stats: %v", err)
		return
	}

	log.Printf("[STATS] Registered users: %d | Active now (15min): %d | Active today (24h): %d | Docs uploaded (24h): %d | Docs downloaded (24h): %d",
		totalUsers, activeNow, activeToday, docStats.Uploaded, docStats.Downloaded)
}

// TrackAPIRequest is called by the analytics middleware; kept for compatibility.
func (s *StatsService) TrackAPIRequest(endpoint string) error {
	return s.db.Exec("INSERT INTO api_requests (endpoint, method, created_at) VALUES (?, '', NOW())", endpoint).Error
}


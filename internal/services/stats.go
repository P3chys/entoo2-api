package services

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type StatsService struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewStatsService creates a new stats service instance
func NewStatsService(db *gorm.DB, cfg *config.Config) (*StatsService, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &StatsService{
		db:    db,
		redis: client,
	}, nil
}

// UserStats contains user-related statistics
type UserStats struct {
	TotalUsers        int64 `json:"total_users"`
	RegisteredToday   int64 `json:"registered_today"`
	ActiveLast24h     int64 `json:"active_last_24h"`
	ActiveLast15min   int64 `json:"active_last_15min"`
	VerifiedUsers     int64 `json:"verified_users"`
	AdminCount        int64 `json:"admin_count"`
}

// GetUserStats returns user-related statistics
func (s *StatsService) GetUserStats() (*UserStats, error) {
	stats := &UserStats{}

	// Total users
	if err := s.db.Model(&models.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	// Users registered today
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.User{}).
		Where("created_at >= ?", today).
		Count(&stats.RegisteredToday).Error; err != nil {
		return nil, err
	}

	// Verified users
	if err := s.db.Model(&models.User{}).
		Where("email_verified = ?", true).
		Count(&stats.VerifiedUsers).Error; err != nil {
		return nil, err
	}

	// Admin count
	if err := s.db.Model(&models.User{}).
		Where("role = ?", models.RoleAdmin).
		Count(&stats.AdminCount).Error; err != nil {
		return nil, err
	}

	// Active users from Redis sorted set
	ctx := context.Background()

	// Active in last 15 minutes
	fifteenMinAgo := float64(time.Now().Add(-15 * time.Minute).Unix())
	now := float64(time.Now().Unix())

	count, err := s.redis.ZCount(ctx, "active_users", fmt.Sprintf("%f", fifteenMinAgo), fmt.Sprintf("%f", now)).Result()
	if err != nil && err != redis.Nil {
		// Log error but continue with 0
		stats.ActiveLast15min = 0
	} else {
		stats.ActiveLast15min = count
	}

	// Active in last 24 hours
	oneDayAgo := float64(time.Now().Add(-24 * time.Hour).Unix())
	count, err = s.redis.ZCount(ctx, "active_users", fmt.Sprintf("%f", oneDayAgo), fmt.Sprintf("%f", now)).Result()
	if err != nil && err != redis.Nil {
		stats.ActiveLast24h = 0
	} else {
		stats.ActiveLast24h = count
	}

	return stats, nil
}

// APIStats contains API usage statistics
type APIStats struct {
	TotalRequests   int64                `json:"total_requests"`
	RequestsToday   int64                `json:"requests_today"`
	TopEndpoints    []EndpointStats      `json:"top_endpoints"`
	RequestsByHour  map[string]int64     `json:"requests_by_hour"`
}

// EndpointStats contains statistics for a single endpoint
type EndpointStats struct {
	Endpoint string `json:"endpoint"`
	Count    int64  `json:"count"`
}

// GetAPIStats returns API usage statistics
func (s *StatsService) GetAPIStats() (*APIStats, error) {
	stats := &APIStats{
		TopEndpoints:   make([]EndpointStats, 0),
		RequestsByHour: make(map[string]int64),
	}

	ctx := context.Background()
	log.Printf("[StatsService] GetAPIStats called")

	// Get all endpoint keys
	pattern := "api:requests:endpoint:*"
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	endpointCounts := make(map[string]int64)
	for _, key := range keys {
		count, err := s.redis.Get(ctx, key).Int64()
		if err != nil && err != redis.Nil {
			continue
		}
		endpoint := strings.TrimPrefix(key, "api:requests:endpoint:")
		endpointCounts[endpoint] = count
		stats.TotalRequests += count
	}

	// Sort endpoints by count and get top 10
	type kv struct {
		Key   string
		Value int64
	}
	var sorted []kv
	for k, v := range endpointCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		stats.TopEndpoints = append(stats.TopEndpoints, EndpointStats{
			Endpoint: sorted[i].Key,
			Count:    sorted[i].Value,
		})
	}

	// Get hourly requests for today
	today := time.Now().Format("2006-01-02")
	log.Printf("[StatsService] Fetching hourly data for date: %s", today)
	for hour := 0; hour < 24; hour++ {
		key := fmt.Sprintf("api:requests:hour:%s:%02d", today, hour)
		count, err := s.redis.Get(ctx, key).Int64()
		if err != nil && err != redis.Nil {
			log.Printf("[StatsService] Error getting key %s: %v", key, err)
			continue
		}
		if count > 0 {
			log.Printf("[StatsService] Found data for hour %d: %d requests", hour, count)
		}
		stats.RequestsByHour[fmt.Sprintf("%02d:00", hour)] = count
	}
	log.Printf("[StatsService] RequestsByHour map has %d entries", len(stats.RequestsByHour))

	// Get today's total
	for _, count := range stats.RequestsByHour {
		stats.RequestsToday += count
	}

	return stats, nil
}

// ActivityStats contains activity-related statistics
type ActivityStats struct {
	TotalActivities  int64                    `json:"total_activities"`
	ActivitiesToday  int64                    `json:"activities_today"`
	ByType           map[string]int64         `json:"by_type"`
	RecentActivities []models.Activity        `json:"recent_activities"`
}

// GetActivityStats returns activity-related statistics
func (s *StatsService) GetActivityStats() (*ActivityStats, error) {
	stats := &ActivityStats{
		ByType: make(map[string]int64),
	}

	// Total activities
	if err := s.db.Model(&models.Activity{}).Count(&stats.TotalActivities).Error; err != nil {
		return nil, err
	}

	// Activities today
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.Activity{}).
		Where("created_at >= ?", today).
		Count(&stats.ActivitiesToday).Error; err != nil {
		return nil, err
	}

	// Activities by type
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

	// Recent activities
	if err := s.db.Preload("User").Preload("Subject").Preload("Document").
		Order("created_at desc").
		Limit(20).
		Find(&stats.RecentActivities).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// SystemStats contains system-wide statistics
type SystemStats struct {
	TotalDocuments     int64 `json:"total_documents"`
	DocumentsToday     int64 `json:"documents_today"`
	TotalSubjects      int64 `json:"total_subjects"`
	TotalSemesters     int64 `json:"total_semesters"`
	TotalFlashcardDecks int64 `json:"total_flashcard_decks"`
	TotalComments      int64 `json:"total_comments"`
	TotalQuestions     int64 `json:"total_questions"`
}

// GetSystemStats returns system-wide statistics
func (s *StatsService) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Total documents
	if err := s.db.Model(&models.Document{}).Count(&stats.TotalDocuments).Error; err != nil {
		return nil, err
	}

	// Documents today
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.Document{}).
		Where("created_at >= ?", today).
		Count(&stats.DocumentsToday).Error; err != nil {
		return nil, err
	}

	// Total subjects
	if err := s.db.Model(&models.Subject{}).Count(&stats.TotalSubjects).Error; err != nil {
		return nil, err
	}

	// Total semesters
	if err := s.db.Model(&models.Semester{}).Count(&stats.TotalSemesters).Error; err != nil {
		return nil, err
	}

	// Total flashcard decks
	if err := s.db.Model(&models.FlashcardDeck{}).Count(&stats.TotalFlashcardDecks).Error; err != nil {
		return nil, err
	}

	// Total comments
	if err := s.db.Model(&models.Comment{}).Count(&stats.TotalComments).Error; err != nil {
		return nil, err
	}

	// Total questions
	if err := s.db.Model(&models.Question{}).Count(&stats.TotalQuestions).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// DashboardOverview contains all dashboard statistics
type DashboardOverview struct {
	Users    *UserStats     `json:"users"`
	API      *APIStats      `json:"api"`
	Activity *ActivityStats `json:"activity"`
	System   *SystemStats   `json:"system"`
}

// GetDashboardOverview returns all statistics for the dashboard
func (s *StatsService) GetDashboardOverview() (*DashboardOverview, error) {
	overview := &DashboardOverview{}

	userStats, err := s.GetUserStats()
	if err != nil {
		return nil, err
	}
	overview.Users = userStats

	apiStats, err := s.GetAPIStats()
	if err != nil {
		return nil, err
	}
	overview.API = apiStats

	activityStats, err := s.GetActivityStats()
	if err != nil {
		return nil, err
	}
	overview.Activity = activityStats

	systemStats, err := s.GetSystemStats()
	if err != nil {
		return nil, err
	}
	overview.System = systemStats

	return overview, nil
}

// TrackActiveUser adds a user to the active users sorted set
func (s *StatsService) TrackActiveUser(userID string) error {
	ctx := context.Background()
	timestamp := float64(time.Now().Unix())

	// Add to sorted set with timestamp as score
	err := s.redis.ZAdd(ctx, "active_users", redis.Z{
		Score:  timestamp,
		Member: userID,
	}).Err()

	if err != nil {
		return err
	}

	// Clean up old entries (older than 24 hours)
	oneDayAgo := float64(time.Now().Add(-24 * time.Hour).Unix())
	s.redis.ZRemRangeByScore(ctx, "active_users", "-inf", strconv.FormatFloat(oneDayAgo, 'f', -1, 64))

	return nil
}

// TrackAPIRequest increments the API request counters
func (s *StatsService) TrackAPIRequest(endpoint string) error {
	ctx := context.Background()
	now := time.Now()

	// Increment endpoint counter
	endpointKey := fmt.Sprintf("api:requests:endpoint:%s", endpoint)
	if err := s.redis.Incr(ctx, endpointKey).Err(); err != nil {
		return err
	}

	// Increment hourly counter
	hourKey := fmt.Sprintf("api:requests:hour:%s:%02d", now.Format("2006-01-02"), now.Hour())
	if err := s.redis.Incr(ctx, hourKey).Err(); err != nil {
		return err
	}

	// Set expiry on hourly key (48 hours to keep data for a day after)
	s.redis.Expire(ctx, hourKey, 48*time.Hour)

	// Increment total daily counter
	dailyKey := fmt.Sprintf("api:requests:daily:%s", now.Format("2006-01-02"))
	if err := s.redis.Incr(ctx, dailyKey).Err(); err != nil {
		return err
	}
	s.redis.Expire(ctx, dailyKey, 7*24*time.Hour) // Keep for 7 days

	return nil
}

// DocumentStats24h contains document upload/download counts for the last 24 hours
type DocumentStats24h struct {
	Uploaded   int64
	Downloaded int64
}

// GetDocumentStats24h queries the activities table for upload and download counts in the last 24h
func (s *StatsService) GetDocumentStats24h() (*DocumentStats24h, error) {
	stats := &DocumentStats24h{}
	cutoff := time.Now().Add(-24 * time.Hour)

	if err := s.db.Model(&models.Activity{}).
		Where("activity_type = ? AND created_at >= ?", models.ActivityDocumentUploaded, cutoff).
		Count(&stats.Uploaded).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&models.Activity{}).
		Where("activity_type = ? AND created_at >= ?", models.ActivityDocumentDownloaded, cutoff).
		Count(&stats.Downloaded).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// StartObservabilityLogger starts a background goroutine that logs app-level stats every 5 minutes
func (s *StatsService) StartObservabilityLogger() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// Log once on startup
		s.logStats()

		for range ticker.C {
			s.logStats()
		}
	}()
	log.Println("[STATS] Observability logger started (interval: 5m)")
}

func (s *StatsService) logStats() {
	// Total registered users
	var totalUsers int64
	if err := s.db.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		log.Printf("[STATS] ERROR: failed to query users: %v", err)
		return
	}

	// Active users from Redis
	ctx := context.Background()
	now := float64(time.Now().Unix())

	fifteenMinAgo := float64(time.Now().Add(-15 * time.Minute).Unix())
	activeNow, err := s.redis.ZCount(ctx, "active_users", fmt.Sprintf("%f", fifteenMinAgo), fmt.Sprintf("%f", now)).Result()
	if err != nil {
		activeNow = 0
	}

	oneDayAgo := float64(time.Now().Add(-24 * time.Hour).Unix())
	activeToday, err := s.redis.ZCount(ctx, "active_users", fmt.Sprintf("%f", oneDayAgo), fmt.Sprintf("%f", now)).Result()
	if err != nil {
		activeToday = 0
	}

	// Document stats
	docStats, err := s.GetDocumentStats24h()
	if err != nil {
		log.Printf("[STATS] ERROR: failed to query document stats: %v", err)
		return
	}

	log.Printf("[STATS] Registered users: %d | Active now (15min): %d | Active today (24h): %d | Docs uploaded (24h): %d | Docs downloaded (24h): %d",
		totalUsers, activeNow, activeToday, docStats.Uploaded, docStats.Downloaded)
}

// Close closes the Redis connection
func (s *StatsService) Close() error {
	return s.redis.Close()
}

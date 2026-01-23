package middleware

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Analytics provides API request tracking functionality using Redis
type Analytics struct {
	redis *redis.Client
}

// NewAnalytics creates a new analytics middleware instance
func NewAnalytics(redisURL string) (*Analytics, error) {
	opt, err := redis.ParseURL(redisURL)
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

	return &Analytics{redis: client}, nil
}

// TrackRequests returns a middleware that tracks API requests
func (a *Analytics) TrackRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip tracking for certain paths
		path := c.FullPath()
		if path == "" || path == "/health" {
			c.Next()
			return
		}

		// Track the request asynchronously to not slow down response
		go a.trackRequest(path, c.Request.Method)

		c.Next()
	}
}

func (a *Analytics) trackRequest(path, method string) {
	ctx := context.Background()
	now := time.Now()

	// Normalize the path (replace IDs with placeholders for aggregation)
	endpoint := fmt.Sprintf("%s %s", method, path)

	// Increment endpoint counter
	endpointKey := fmt.Sprintf("api:requests:endpoint:%s", endpoint)
	if err := a.redis.Incr(ctx, endpointKey).Err(); err != nil {
		log.Printf("Analytics: failed to track endpoint: %v", err)
	}

	// Increment hourly counter
	hourKey := fmt.Sprintf("api:requests:hour:%s:%02d", now.Format("2006-01-02"), now.Hour())
	if err := a.redis.Incr(ctx, hourKey).Err(); err != nil {
		log.Printf("Analytics: failed to track hourly: %v", err)
	}
	// Set expiry (48 hours)
	a.redis.Expire(ctx, hourKey, 48*time.Hour)

	// Increment total daily counter
	dailyKey := fmt.Sprintf("api:requests:daily:%s", now.Format("2006-01-02"))
	if err := a.redis.Incr(ctx, dailyKey).Err(); err != nil {
		log.Printf("Analytics: failed to track daily: %v", err)
	}
	// Set expiry (7 days)
	a.redis.Expire(ctx, dailyKey, 7*24*time.Hour)

	// Increment total counter
	if err := a.redis.Incr(ctx, "api:requests:total").Err(); err != nil {
		log.Printf("Analytics: failed to track total: %v", err)
	}
}

// Close closes the Redis connection
func (a *Analytics) Close() error {
	return a.redis.Close()
}

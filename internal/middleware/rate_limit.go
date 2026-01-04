package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter provides rate limiting functionality using Redis
type RateLimiter struct {
	redis *redis.Client
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(redisURL string) (*RateLimiter, error) {
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

	return &RateLimiter{redis: client}, nil
}

// RateLimitByIP creates a middleware that limits requests by IP address
// maxRequests: maximum number of requests allowed
// window: time window in seconds (e.g., 3600 for 1 hour)
func (rl *RateLimiter) RateLimitByIP(maxRequests int, window int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:ip:%s:%s", c.FullPath(), ip)

		ctx := context.Background()

		// Increment counter
		count, err := rl.redis.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, allow the request but log the error
			_ = c.Error(fmt.Errorf("rate limiter error: %w", err))
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			rl.redis.Expire(ctx, key, time.Duration(window)*time.Second)
		}

		// Check if limit exceeded
		if count > int64(maxRequests) {
			// Get TTL for Retry-After header
			ttl, _ := rl.redis.TTL(ctx, key).Result()

			c.Header("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please try again later.",
				},
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", maxRequests-int(count)))

		c.Next()
	}
}

// RateLimitByEmail creates a middleware that limits requests by email address
// This is useful for preventing spam to specific email addresses
func (rl *RateLimiter) RateLimitByEmail(maxRequests int, window int, emailField string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body to get email
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": err.Error(),
				},
			})
			c.Abort()
			return
		}

		email, ok := body[emailField].(string)
		if !ok || email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Email is required",
				},
			})
			c.Abort()
			return
		}

		// Set email in context for use by handler
		c.Set("rate_limit_email", email)

		key := fmt.Sprintf("rate_limit:email:%s:%s", c.FullPath(), email)

		ctx := context.Background()

		// Increment counter
		count, err := rl.redis.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, allow the request but log the error
			_ = c.Error(fmt.Errorf("rate limiter error: %w", err))
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			rl.redis.Expire(ctx, key, time.Duration(window)*time.Second)
		}

		// Check if limit exceeded
		if count > int64(maxRequests) {
			// Get TTL for Retry-After header
			ttl, _ := rl.redis.TTL(ctx, key).Result()

			c.Header("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests for this email. Please try again later.",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Close closes the Redis connection
func (rl *RateLimiter) Close() error {
	return rl.redis.Close()
}

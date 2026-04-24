package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter provides in-memory fixed-window rate limiting.
// State is lost on server restart, which is acceptable for this use case.
type RateLimiter struct {
	windows sync.Map // key(string) → *windowEntry
	done    chan struct{}
}

type windowEntry struct {
	count   atomic.Int64
	resetAt time.Time
	mu      sync.Mutex
}

// NewRateLimiter creates a rate limiter and starts a background cleanup goroutine.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{done: make(chan struct{})}
	go rl.gcWorker()
	return rl
}

// gcWorker periodically removes expired window entries to prevent unbounded growth.
func (rl *RateLimiter) gcWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.windows.Range(func(k, v interface{}) bool {
				if e, ok := v.(*windowEntry); ok {
					e.mu.Lock()
					expired := now.After(e.resetAt)
					e.mu.Unlock()
					if expired {
						rl.windows.Delete(k)
					}
				}
				return true
			})
		case <-rl.done:
			return
		}
	}
}

func (rl *RateLimiter) getEntry(key string, window time.Duration) *windowEntry {
	now := time.Now()
	v, _ := rl.windows.LoadOrStore(key, &windowEntry{resetAt: now.Add(window)})
	e := v.(*windowEntry)

	e.mu.Lock()
	if now.After(e.resetAt) {
		e.count.Store(0)
		e.resetAt = now.Add(window)
	}
	e.mu.Unlock()

	return e
}

// RateLimitByIP limits requests per client IP + route path.
func (rl *RateLimiter) RateLimitByIP(maxRequests int, windowSecs int) gin.HandlerFunc {
	window := time.Duration(windowSecs) * time.Second
	return func(c *gin.Context) {
		key := fmt.Sprintf("ip:%s:%s", c.FullPath(), c.ClientIP())
		e := rl.getEntry(key, window)
		count := e.count.Add(1)

		if count > int64(maxRequests) {
			e.mu.Lock()
			retryAfter := int(time.Until(e.resetAt).Seconds())
			e.mu.Unlock()

			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
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

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", maxRequests-int(count)))
		c.Next()
	}
}

// RateLimitByEmail limits requests per email address + route path.
func (rl *RateLimiter) RateLimitByEmail(maxRequests int, windowSecs int, emailField string) gin.HandlerFunc {
	window := time.Duration(windowSecs) * time.Second
	return func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   gin.H{"code": "VALIDATION_ERROR", "message": err.Error()},
			})
			c.Abort()
			return
		}

		email, ok := body[emailField].(string)
		if !ok || email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   gin.H{"code": "VALIDATION_ERROR", "message": "Email is required"},
			})
			c.Abort()
			return
		}

		c.Set("rate_limit_email", email)

		key := fmt.Sprintf("email:%s:%s", c.FullPath(), email)
		e := rl.getEntry(key, window)
		count := e.count.Add(1)

		if count > int64(maxRequests) {
			e.mu.Lock()
			retryAfter := int(time.Until(e.resetAt).Seconds())
			e.mu.Unlock()

			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
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

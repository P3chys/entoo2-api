package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		log.Printf("[DEBUG AUTH] Path: %s, AuthHeader: %s", c.Request.URL.Path, authHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Authorization header required",
				},
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid authorization format",
				},
			})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid token claims",
				},
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims["user_id"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != string(models.RoleAdmin) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Admin access required",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ActiveUserTracker tracks active users in Redis
type ActiveUserTracker struct {
	redis *redis.Client
}

// NewActiveUserTracker creates a new active user tracker
func NewActiveUserTracker(redisURL string) (*ActiveUserTracker, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &ActiveUserTracker{redis: client}, nil
}

// TrackActiveUsers returns a middleware that tracks active users
func (t *ActiveUserTracker) TrackActiveUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by AuthRequired)
		userID, exists := c.Get("user_id")
		if exists && userID != nil {
			// Track the user asynchronously to not slow down response
			go t.trackUser(userID.(string))
		}
		c.Next()
	}
}

func (t *ActiveUserTracker) trackUser(userID string) {
	ctx := context.Background()
	timestamp := float64(time.Now().Unix())

	// Add to sorted set with timestamp as score
	err := t.redis.ZAdd(ctx, "active_users", redis.Z{
		Score:  timestamp,
		Member: userID,
	}).Err()

	if err != nil {
		log.Printf("ActiveUserTracker: failed to track user: %v", err)
		return
	}

	// Clean up old entries (older than 24 hours) - do this periodically
	// Only clean up 1% of the time to avoid overhead
	if time.Now().UnixNano()%100 == 0 {
		oneDayAgo := float64(time.Now().Add(-24 * time.Hour).Unix())
		t.redis.ZRemRangeByScore(ctx, "active_users", "-inf", strconv.FormatFloat(oneDayAgo, 'f', -1, 64))
	}
}

// Close closes the Redis connection
func (t *ActiveUserTracker) Close() error {
	return t.redis.Close()
}

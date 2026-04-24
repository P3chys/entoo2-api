package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// extractToken extracts the Bearer token from the Authorization header.
func extractToken(c *gin.Context) string {
	return strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
}

// TokenHash returns the hex-encoded SHA-256 of the raw JWT string.
// Used as the primary key in the revoked_tokens table.
func TokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func AuthRequired(cfg *config.Config, db ...*gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "Authorization header required"},
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "Invalid authorization format"},
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
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "Invalid or expired token"},
			})
			c.Abort()
			return
		}

		// Check token blacklist in DB
		if len(db) > 0 && db[0] != nil {
			jti := TokenHash(tokenString)
			var count int64
			db[0].Raw(
				"SELECT COUNT(*) FROM revoked_tokens WHERE jti = ? AND expires_at > NOW()",
				jti,
			).Scan(&count)
			if count > 0 {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   gin.H{"code": "UNAUTHORIZED", "message": "Token revoked"},
				})
				c.Abort()
				return
			}
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "Invalid token claims"},
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
		roleInterface, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   gin.H{"code": "FORBIDDEN", "message": "Admin access required"},
			})
			c.Abort()
			return
		}
		roleStr, ok := roleInterface.(string)
		if !ok || roleStr != string(models.RoleAdmin) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   gin.H{"code": "FORBIDDEN", "message": "Admin access required"},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Analytics tracks API requests by writing to the api_requests table.
type Analytics struct {
	db *gorm.DB
}

func NewAnalytics(db *gorm.DB) *Analytics {
	return &Analytics{db: db}
}

func (a *Analytics) TrackRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" || path == "/health" {
			c.Next()
			return
		}
		go a.trackRequest(path, c.Request.Method)
		c.Next()
	}
}

func (a *Analytics) trackRequest(path, method string) {
	if err := a.db.Exec(
		"INSERT INTO api_requests (endpoint, method, created_at) VALUES (?, ?, NOW())",
		method+" "+path, method,
	).Error; err != nil {
		log.Printf("Analytics: failed to record request: %v", err)
	}
}

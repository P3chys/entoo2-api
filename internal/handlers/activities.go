package handlers

import (
	"net/http"
	"strconv"

	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/gin-gonic/gin"
)

func GetRecentActivities(activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		if limit > 50 {
			limit = 50
		}

		activities, err := activity.GetRecentActivities(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch activities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": activities})
	}
}

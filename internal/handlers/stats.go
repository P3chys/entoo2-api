package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/gin-gonic/gin"
)

// GetStatsOverview returns all dashboard statistics
func GetStatsOverview(statsService *services.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		overview, err := statsService.GetDashboardOverview()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to get dashboard overview",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    overview,
		})
	}
}

// GetUserStats returns user-related statistics
func GetUserStats(statsService *services.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := statsService.GetUserStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to get user statistics",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	}
}

// GetAPIStats returns API usage statistics
func GetAPIStats(statsService *services.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := statsService.GetAPIStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to get API statistics",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	}
}

// GetActivityStats returns activity-related statistics
func GetActivityStats(statsService *services.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := statsService.GetActivityStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to get activity statistics",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	}
}

// GetSystemStats returns system-wide statistics
func GetSystemStats(statsService *services.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := statsService.GetSystemStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to get system statistics",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stats,
		})
	}
}

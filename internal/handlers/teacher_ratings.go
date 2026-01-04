package handlers

import (
	"fmt"
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RateTeacherRequest struct {
	Rating int `json:"rating" binding:"required,min=1,max=5"`
}

// RateTeacher creates or updates a teacher rating
func RateTeacher(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teacherIDStr := c.Param("id")
		teacherID, err := uuid.Parse(teacherIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid teacher ID"})
			return
		}

		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		var req RateTeacherRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify teacher exists
		var teacher models.SubjectTeacher
		if err := db.First(&teacher, "id = ?", teacherID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// Check if rating already exists
		var existingRating models.TeacherRating
		err = db.Where("subject_teacher_id = ? AND user_id = ?", teacherID, userID).
			First(&existingRating).Error

		statusCode := http.StatusCreated
		message := "Rating created successfully"

		if err == nil {
			// Update existing rating
			existingRating.Rating = req.Rating
			if err := db.Save(&existingRating).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rating"})
				return
			}
			statusCode = http.StatusOK
			message = "Rating updated successfully"
		} else if err == gorm.ErrRecordNotFound {
			// Create new rating
			newRating := models.TeacherRating{
				SubjectTeacherID: teacherID,
				UserID:           userID,
				Rating:           req.Rating,
			}
			if err := db.Create(&newRating).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rating"})
				return
			}
			existingRating = newRating
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		c.JSON(statusCode, gin.H{
			"success": true,
			"data":    existingRating,
			"message": message,
		})
	}
}

// DeleteTeacherRating removes a user's rating for a teacher
func DeleteTeacherRating(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teacherIDStr := c.Param("id")
		teacherID, err := uuid.Parse(teacherIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid teacher ID"})
			return
		}

		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		result := db.Where("subject_teacher_id = ? AND user_id = ?", teacherID, userID).
			Delete(&models.TeacherRating{})

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rating"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rating not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Rating deleted successfully",
		})
	}
}

// GetTeacherRatings returns aggregate rating data for a teacher
func GetTeacherRatings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teacherIDStr := c.Param("id")
		teacherID, err := uuid.Parse(teacherIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid teacher ID"})
			return
		}

		userIDStr := c.GetString("user_id")

		// Get average rating and count
		var result struct {
			AverageRating float64
			TotalRatings  int64
		}

		err = db.Model(&models.TeacherRating{}).
			Select("COALESCE(AVG(rating), 0) as average_rating, COUNT(*) as total_ratings").
			Where("subject_teacher_id = ?", teacherID).
			Scan(&result).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ratings"})
			return
		}

		// Get rating distribution
		var distribution []struct {
			Rating int
			Count  int64
		}
		db.Model(&models.TeacherRating{}).
			Select("rating, COUNT(*) as count").
			Where("subject_teacher_id = ?", teacherID).
			Group("rating").
			Scan(&distribution)

		distMap := make(map[string]int64)
		for i := 1; i <= 5; i++ {
			distMap[fmt.Sprintf("%d", i)] = 0
		}
		for _, d := range distribution {
			distMap[fmt.Sprintf("%d", d.Rating)] = d.Count
		}

		// Get user's rating
		var userRating *int
		var ur models.TeacherRating
		if err := db.Where("subject_teacher_id = ? AND user_id = ?", teacherID, userIDStr).
			First(&ur).Error; err == nil {
			userRating = &ur.Rating
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"average_rating":      result.AverageRating,
				"total_ratings":       result.TotalRatings,
				"user_rating":         userRating,
				"rating_distribution": distMap,
			},
		})
	}
}

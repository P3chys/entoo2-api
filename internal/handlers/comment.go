package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateCommentRequest struct {
	Content     string `json:"content" binding:"required"`
	IsAnonymous bool   `json:"is_anonymous"`
}

func CreateComment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.Param("id")
		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subject ID"})
			return
		}

		var req CreateCommentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid user ID"})
			return
		}

		comment := models.Comment{
			SubjectID:   subjectID,
			UserID:      userID,
			Content:     req.Content,
			IsAnonymous: req.IsAnonymous,
		}

		if result := db.Create(&comment); result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
			return
		}

		// Fetch created comment with user details
		var createdComment models.Comment
		if err := db.Preload("User").First(&createdComment, comment.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created comment"})
			return
		}

		// Sanitize user if anonymous
		if createdComment.IsAnonymous {
			// Create a copy or modify the struct fields to hide user info in response
			// Note: modifying struct directly works for JSON serialization
			createdComment.User.DisplayName = "Anonymous Student"
			createdComment.User.Email = "" // Hide email
			// We might want to zero out the UserID in the response too if strictly anonymous, 
			// but frontend uses it for "is my comment" check. 
			// Ideally we return a DTO, but for now this matches the logic.
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": createdComment})
	}
}

func GetCommentsBySubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.Param("id")
		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subject ID"})
			return
		}

		var comments []models.Comment
		if result := db.Where("subject_id = ?", subjectID).Preload("User").Order("created_at desc").Find(&comments); result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
			return
		}

		// Sanitize anonymous comments
		for i := range comments {
			if comments[i].IsAnonymous {
				comments[i].User.DisplayName = "Anonymous Student"
				comments[i].User.Email = ""
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": comments})
	}
}

func DeleteComment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		commentIDStr := c.Param("id")
		commentID, err := uuid.Parse(commentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment ID"})
			return
		}

		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}
		// Optionally check if user is admin here

		var comment models.Comment
		if err := db.First(&comment, commentID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
			return
		}

		if comment.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this comment"})
			return
		}

		if err := db.Delete(&comment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

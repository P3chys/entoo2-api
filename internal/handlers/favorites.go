package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ToggleFavoriteSubject adds or removes a subject from user's favorites
func ToggleFavoriteSubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		subjectID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subject ID"})
			return
		}

		// Check if subject exists
		var subject models.Subject
		if err := db.First(&subject, "id = ?", subjectID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subject not found"})
			return
		}

		// Check if exists in association
		var count int64
		err = db.Table("user_favorite_subjects").
			Where("user_id = ? AND subject_id = ?", userID, subjectID).
			Count(&count).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if count > 0 {
			// Remove favorite
			err = db.Exec("DELETE FROM user_favorite_subjects WHERE user_id = ? AND subject_id = ?", userID, subjectID).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove favorite"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "is_favorite": false})
		} else {
			// Add favorite
			err = db.Exec("INSERT INTO user_favorite_subjects (user_id, subject_id) VALUES (?, ?)", userID, subjectID).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add favorite"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "is_favorite": true})
		}
	}
}

// ToggleFavoriteDocument adds or removes a document from user's favorites
func ToggleFavoriteDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		documentID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
			return
		}

		// Check if document exists
		var document models.Document
		if err := db.First(&document, "id = ?", documentID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
			return
		}

		// Check if exists in association
		var count int64
		err = db.Table("user_favorite_documents").
			Where("user_id = ? AND document_id = ?", userID, documentID).
			Count(&count).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if count > 0 {
			// Remove favorite
			err = db.Exec("DELETE FROM user_favorite_documents WHERE user_id = ? AND document_id = ?", userID, documentID).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove favorite"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "is_favorite": false})
		} else {
			// Add favorite
			err = db.Exec("INSERT INTO user_favorite_documents (user_id, document_id) VALUES (?, ?)", userID, documentID).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add favorite"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "is_favorite": true})
		}
	}
}

func ListFavorites(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetString("user_id")

		var subjects []models.Subject
		var documents []models.Document

		// Fetch favorite subjects
		// We explicitly want ONLY favorites, so we INNER JOIN
		// Use Select to compute is_favorite in SQL to avoid GORM trying to select it as a physical column
		err := db.Select("subjects.*, true as is_favorite").
			Joins("JOIN user_favorite_subjects ufs ON subjects.id = ufs.subject_id AND ufs.user_id = ?", userIDStr).
			Preload("Semester").
			Find(&subjects).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch favorite subjects"})
			return
		}

		// Fetch favorite documents
		// Use Select to compute is_favorite in SQL to avoid GORM trying to select it as a physical column
		err = db.Select("documents.*, true as is_favorite").
			Joins("JOIN user_favorite_documents ufd ON documents.id = ufd.document_id AND ufd.user_id = ?", userIDStr).
			Preload("Uploader").Preload("Subject").
			Find(&documents).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch favorite documents"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true, 
			"data": gin.H{
				"subjects": subjects,
				"documents": documents,
			},
		})
	}
}

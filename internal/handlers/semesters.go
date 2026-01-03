package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Request/Response types
type CreateSemesterRequest struct {
	NameCS     string `json:"name_cs" binding:"required"`
	OrderIndex int    `json:"order_index"`
}

type UpdateSemesterRequest struct {
	NameCS     *string `json:"name_cs"`
	OrderIndex *int    `json:"order_index"`
}

// ListSemesters returns all semesters ordered by order_index
func ListSemesters(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var semesters []models.Semester
		if err := db.Order("order_index asc").Find(&semesters).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch semesters",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    semesters,
		})
	}
}

// GetSemester returns a single semester with its subjects
func GetSemester(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		semesterID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid semester ID format",
				},
			})
			return
		}

		var semester models.Semester
		if err := db.Preload("Subjects").First(&semester, "id = ?", semesterID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Semester not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch semester",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    semester,
		})
	}
}

// CreateSemester creates a new semester (admin only)
func CreateSemester(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateSemesterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": err.Error(),
				},
			})
			return
		}

		semester := models.Semester{
			NameCS:     req.NameCS,
			OrderIndex: req.OrderIndex,
		}

		if err := db.Create(&semester).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create semester",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    semester,
		})
	}
}

// UpdateSemester updates an existing semester (admin only)
func UpdateSemester(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		semesterID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid semester ID format",
				},
			})
			return
		}

		var semester models.Semester
		if err := db.First(&semester, "id = ?", semesterID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Semester not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch semester",
				},
			})
			return
		}

		var req UpdateSemesterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": err.Error(),
				},
			})
			return
		}

		// Update fields if provided
		if req.NameCS != nil {
			semester.NameCS = *req.NameCS
		}
		if req.OrderIndex != nil {
			semester.OrderIndex = *req.OrderIndex
		}

		if err := db.Save(&semester).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update semester",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    semester,
		})
	}
}

// DeleteSemester deletes a semester (admin only)
func DeleteSemester(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		semesterID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid semester ID format",
				},
			})
			return
		}

		// Check if semester has subjects
		var subjectCount int64
		db.Model(&models.Subject{}).Where("semester_id = ?", semesterID).Count(&subjectCount)
		if subjectCount > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "HAS_SUBJECTS",
					"message": "Cannot delete semester with existing subjects",
				},
			})
			return
		}

		result := db.Delete(&models.Semester{}, "id = ?", semesterID)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete semester",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Semester not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Semester deleted successfully",
		})
	}
}

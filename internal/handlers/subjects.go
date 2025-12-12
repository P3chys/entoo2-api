package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Request/Response types
type TeacherRequest struct {
	Name    string `json:"name" binding:"required"`
	TopicCS string `json:"topic_cs"`
	TopicEN string `json:"topic_en"`
}

type CreateSubjectRequest struct {
	SemesterID    string           `json:"semester_id" binding:"required"`
	NameCS        string           `json:"name_cs" binding:"required"`
	NameEN        string           `json:"name_en" binding:"required"`
	Code          string           `json:"code" binding:"required,min=3,max=6"`
	DescriptionCS string           `json:"description_cs"`
	DescriptionEN string           `json:"description_en"`
	Credits       int              `json:"credits"`
	Teachers      []TeacherRequest `json:"teachers"`
}

type UpdateSubjectRequest struct {
	SemesterID    *string           `json:"semester_id"`
	NameCS        *string           `json:"name_cs"`
	NameEN        *string           `json:"name_en"`
	Code          *string           `json:"code"`
	DescriptionCS *string           `json:"description_cs"`
	DescriptionEN *string           `json:"description_en"`
	Credits       *int              `json:"credits"`
	Teachers      *[]TeacherRequest `json:"teachers"`
}

// ListSubjects returns all subjects with semester info
func ListSubjects(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var subjects []models.Subject

		query := db.Preload("Semester")

		// Optional filter by semester
		if semesterID := c.Query("semester_id"); semesterID != "" {
			query = query.Where("semester_id = ?", semesterID)
		}

		if err := query.Find(&subjects).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch subjects",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    subjects,
		})
	}
}

// GetSubject returns a single subject with details
func GetSubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		subjectID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid subject ID format",
				},
			})
			return
		}

		var subject models.Subject
		if err := db.Preload("Semester").Preload("Teachers").First(&subject, "id = ?", subjectID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Subject not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch subject",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    subject,
		})
	}
}

// CreateSubject creates a new subject (admin only)
func CreateSubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateSubjectRequest
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

		semesterID, err := uuid.Parse(req.SemesterID)
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

		// Verify semester exists
		var semester models.Semester
		if err := db.First(&semester, "id = ?", semesterID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_SEMESTER",
					"message": "Semester not found",
				},
			})
			return
		}

		subject := models.Subject{
			SemesterID:    semesterID,
			NameCS:        req.NameCS,
			NameEN:        req.NameEN,
			Code:          req.Code,
			DescriptionCS: req.DescriptionCS,
			DescriptionEN: req.DescriptionEN,
			Credits:       req.Credits,
		}

		if len(req.Teachers) > 0 {
			subject.Teachers = make([]models.SubjectTeacher, len(req.Teachers))
			for i, t := range req.Teachers {
				subject.Teachers[i] = models.SubjectTeacher{
					TeacherName: t.Name,
					TopicCS:     t.TopicCS,
					TopicEN:     t.TopicEN,
				}
			}
		}

		if err := db.Create(&subject).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create subject",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    subject,
		})
	}
}

// UpdateSubject updates an existing subject (admin only)
func UpdateSubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		subjectID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid subject ID format",
				},
			})
			return
		}

		var subject models.Subject
		if err := db.First(&subject, "id = ?", subjectID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Subject not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch subject",
				},
			})
			return
		}

		var req UpdateSubjectRequest
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
		if req.SemesterID != nil {
			semesterID, err := uuid.Parse(*req.SemesterID)
			if err == nil {
				// Verify semester exists
				var semester models.Semester
				if err := db.First(&semester, "id = ?", semesterID).Error; err == nil {
					subject.SemesterID = semesterID
				}
			}
		}
		if req.NameCS != nil {
			subject.NameCS = *req.NameCS
		}
		if req.NameEN != nil {
			subject.NameEN = *req.NameEN
		}
		if req.DescriptionCS != nil {
			subject.DescriptionCS = *req.DescriptionCS
		}
		if req.DescriptionEN != nil {
			subject.DescriptionEN = *req.DescriptionEN
		}
		if req.Code != nil {
			subject.Code = *req.Code
		}
		if req.Credits != nil {
			subject.Credits = *req.Credits
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if req.Teachers != nil {
				// Delete existing teachers
				if err := tx.Delete(&models.SubjectTeacher{}, "subject_id = ?", subject.ID).Error; err != nil {
					return err
				}
				// Add new teachers
				if len(*req.Teachers) > 0 {
					newTeachers := make([]models.SubjectTeacher, len(*req.Teachers))
					for i, t := range *req.Teachers {
						newTeachers[i] = models.SubjectTeacher{
							SubjectID:   subject.ID,
							TeacherName: t.Name,
							TopicCS:     t.TopicCS,
							TopicEN:     t.TopicEN,
						}
					}
					if err := tx.Create(&newTeachers).Error; err != nil {
						return err
					}
				}
			}
			return tx.Save(&subject).Error
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update subject",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    subject,
		})
	}
}

// DeleteSubject deletes a subject (admin only)
func DeleteSubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		subjectID, err := uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid subject ID format",
				},
			})
			return
		}

		// Delete associated teachers first
		db.Where("subject_id = ?", subjectID).Delete(&models.SubjectTeacher{})

		result := db.Delete(&models.Subject{}, "id = ?", subjectID)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete subject",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Subject not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Subject deleted successfully",
		})
	}
}

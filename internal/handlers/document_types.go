package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateDocumentTypeRequest defines the request body for creating a document type
type CreateDocumentTypeRequest struct {
	NameCS string `json:"name_cs" binding:"required,max=100"`
	Icon   string `json:"icon" binding:"omitempty,max=50"`
}

// UpdateDocumentTypeRequest defines the request body for updating a document type
type UpdateDocumentTypeRequest struct {
	NameCS     *string `json:"name_cs" binding:"omitempty,max=100"`
	Icon       *string `json:"icon" binding:"omitempty,max=50"`
	OrderIndex *int    `json:"order_index"`
}

// ReorderDocumentTypeItem represents a single type in the reorder request
type ReorderDocumentTypeItem struct {
	ID         string `json:"id" binding:"required"`
	OrderIndex int    `json:"order_index"`
}

// ReorderDocumentTypesRequest defines the request body for reordering types
type ReorderDocumentTypesRequest struct {
	Types []ReorderDocumentTypeItem `json:"types" binding:"required,min=1"`
}

// ListDocumentTypes lists all document types for a subject
// GET /api/v1/subjects/:id/types
func ListDocumentTypes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		if subjectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Subject ID is required"})
			return
		}

		subjectUUID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid subject ID"})
			return
		}

		// Ensure default types exist for this subject (for migration of existing subjects)
		if err := EnsureDefaultTypesForSubject(db, subjectUUID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to ensure document types"})
			return
		}

		var types []models.DocumentType
		if err := db.Where("subject_id = ?", subjectUUID).
			Order("order_index ASC, created_at ASC").
			Find(&types).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch document types"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    types,
		})
	}
}

// CreateDocumentType creates a new document type for a subject (admin only)
// POST /api/v1/admin/subjects/:id/types
func CreateDocumentType(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		if subjectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Subject ID is required"})
			return
		}

		subjectUUID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid subject ID"})
			return
		}

		// Verify subject exists
		var subject models.Subject
		if err := db.First(&subject, "id = ?", subjectUUID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Subject not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Parse request
		var req CreateDocumentTypeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Check for duplicate type name within the same subject
		var existingType models.DocumentType
		if err := db.Where("subject_id = ? AND name_cs = ?", subjectUUID, req.NameCS).
			First(&existingType).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "A document type with this name already exists for this subject",
			})
			return
		}

		// Get next order index
		var maxOrder int
		db.Model(&models.DocumentType{}).
			Where("subject_id = ?", subjectUUID).
			Select("COALESCE(MAX(order_index), -1)").
			Scan(&maxOrder)

		// Set defaults
		icon := "folder"
		if req.Icon != "" {
			icon = req.Icon
		}

		// Create type
		docType := models.DocumentType{
			SubjectID:  subjectUUID,
			NameCS:     req.NameCS,
			Icon:       icon,
			OrderIndex: maxOrder + 1,
		}

		if err := db.Create(&docType).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create document type"})
			return
		}

		// Create default "Unassigned" category for this type
		defaultCategory := models.DocumentCategory{
			SubjectID:  subjectUUID,
			TypeID:     &docType.ID,
			NameCS:     "Nepřiřazeno",
			OrderIndex: 0,
			CreatedBy:  uuid.Nil, // System-created
		}
		db.Create(&defaultCategory)

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    docType,
		})
	}
}

// UpdateDocumentType updates a document type (admin only)
// PUT /api/v1/admin/types/:id
func UpdateDocumentType(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		typeID := c.Param("id")
		if typeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Type ID is required"})
			return
		}

		typeUUID, err := uuid.Parse(typeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
			return
		}

		// Parse request
		var req UpdateDocumentTypeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Find type
		var docType models.DocumentType
		if err := db.First(&docType, "id = ?", typeUUID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document type not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Update fields
		updates := make(map[string]interface{})
		if req.NameCS != nil {
			updates["name_cs"] = *req.NameCS
		}
		if req.Icon != nil {
			updates["icon"] = *req.Icon
		}
		if req.OrderIndex != nil {
			updates["order_index"] = *req.OrderIndex
		}

		if len(updates) > 0 {
			if err := db.Model(&docType).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update document type"})
				return
			}
		}

		// Fetch updated type
		if err := db.First(&docType, "id = ?", typeUUID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch updated type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    docType,
		})
	}
}

// DeleteDocumentType deletes a document type and all its categories/documents (admin only)
// DELETE /api/v1/admin/types/:id
func DeleteDocumentType(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		typeID := c.Param("id")
		if typeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Type ID is required"})
			return
		}

		typeUUID, err := uuid.Parse(typeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
			return
		}

		// Find type
		var docType models.DocumentType
		if err := db.First(&docType, "id = ?", typeUUID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document type not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Check if there are documents using this type
		var docCount int64
		db.Model(&models.Document{}).Where("type_id = ?", typeUUID).Count(&docCount)
		if docCount > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Cannot delete type with existing documents. Move or delete documents first.",
			})
			return
		}

		// Delete all categories for this type
		if err := db.Where("type_id = ?", typeUUID).Delete(&models.DocumentCategory{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete categories"})
			return
		}

		// Delete the type
		if err := db.Delete(&docType).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete document type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Document type deleted successfully",
		})
	}
}

// ReorderDocumentTypes updates the order of multiple types (admin only)
// PUT /api/v1/admin/types/reorder
func ReorderDocumentTypes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ReorderDocumentTypesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		for _, item := range req.Types {
			typeUUID, err := uuid.Parse(item.ID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Invalid type ID: " + item.ID,
				})
				return
			}

			if err := db.Model(&models.DocumentType{}).
				Where("id = ?", typeUUID).
				Update("order_index", item.OrderIndex).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to update type order",
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Document types reordered successfully",
		})
	}
}

// EnsureDefaultTypesForSubject creates default document types for a subject if none exist
func EnsureDefaultTypesForSubject(db *gorm.DB, subjectID uuid.UUID) error {
	var count int64
	db.Model(&models.DocumentType{}).Where("subject_id = ?", subjectID).Count(&count)

	if count > 0 {
		return nil // Types already exist
	}

	// Create default types
	defaults := models.DefaultDocumentTypes()
	for _, dt := range defaults {
		docType := models.DocumentType{
			SubjectID:  subjectID,
			NameCS:     dt.NameCS,
			Icon:       dt.Icon,
			OrderIndex: dt.Order,
		}
		if err := db.Create(&docType).Error; err != nil {
			return err
		}

		// Create default "Unassigned" category for this type
		defaultCategory := models.DocumentCategory{
			SubjectID:  subjectID,
			TypeID:     &docType.ID,
			NameCS:     "Nepřiřazeno",
			OrderIndex: 0,
			CreatedBy:  uuid.Nil,
		}
		if err := db.Create(&defaultCategory).Error; err != nil {
			return err
		}
	}

	return nil
}

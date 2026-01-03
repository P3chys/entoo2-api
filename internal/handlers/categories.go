package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateCategoryRequest defines the request body for creating a category
type CreateCategoryRequest struct {
	Type   string `json:"type" binding:"required,oneof=lecture seminar other"`
	NameCS string `json:"name_cs" binding:"required,max=200"`
	NameEN string `json:"name_en" binding:"omitempty,max=200"`
}

// UpdateCategoryRequest defines the request body for updating a category
type UpdateCategoryRequest struct {
	NameCS     *string `json:"name_cs" binding:"omitempty,max=200"`
	NameEN     *string `json:"name_en" binding:"omitempty,max=200"`
	OrderIndex *int    `json:"order_index"`
}

// ReorderCategoryItem represents a single category in the reorder request
type ReorderCategoryItem struct {
	ID         string `json:"id" binding:"required"`
	OrderIndex int    `json:"order_index" binding:"required"`
}

// ReorderCategoriesRequest defines the request body for reordering categories
type ReorderCategoriesRequest struct {
	Categories []ReorderCategoryItem `json:"categories" binding:"required,min=1"`
}

// CreateCategory creates a new document category (admin only)
// POST /api/v1/admin/subjects/:id/categories
func CreateCategory(db *gorm.DB) gin.HandlerFunc {
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
		var req CreateCategoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Get user ID from context
		userID, _ := c.Get("user_id")
		userUUID, err := uuid.Parse(userID.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid user"})
			return
		}

		// Check for duplicate category name within the same subject and type
		var existingCategory models.DocumentCategory
		query := db.Where("subject_id = ? AND type = ? AND name_cs = ?", subjectUUID, req.Type, req.NameCS)

		// Only check name_en if it's provided
		if req.NameEN != "" {
			query = query.Or("subject_id = ? AND type = ? AND name_en = ?", subjectUUID, req.Type, req.NameEN)
		}

		err = query.First(&existingCategory).Error

		if err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "A category with this name already exists for this subject and type",
			})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			return
		}

		// Get next order index
		var maxOrder int
		db.Model(&models.DocumentCategory{}).
			Where("subject_id = ? AND type = ?", subjectUUID, req.Type).
			Select("COALESCE(MAX(order_index), -1)").
			Scan(&maxOrder)

		// Create category
		category := models.DocumentCategory{
			SubjectID:  subjectUUID,
			Type:       req.Type,
			NameCS:     req.NameCS,
			NameEN:     req.NameEN,
			OrderIndex: maxOrder + 1,
			CreatedBy:  userUUID,
		}

		if err := db.Create(&category).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create category"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    category,
		})
	}
}

// ListCategories lists all categories for a subject
// GET /api/v1/subjects/:id/categories
func ListCategories(db *gorm.DB) gin.HandlerFunc {
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

		// Optional filter by type
		categoryType := c.Query("type")

		query := db.Where("subject_id = ?", subjectUUID)
		if categoryType != "" {
			if categoryType != "lecture" && categoryType != "seminar" && categoryType != "other" {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type"})
				return
			}
			query = query.Where("type = ?", categoryType)
		}

		var categories []models.DocumentCategory
		if err := query.Order("order_index ASC, created_at ASC").Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch categories"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    categories,
		})
	}
}

// UpdateCategory updates a category (admin only)
// PUT /api/v1/admin/categories/:id
func UpdateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		categoryID := c.Param("id")
		if categoryID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Category ID is required"})
			return
		}

		categoryUUID, err := uuid.Parse(categoryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid category ID"})
			return
		}

		// Parse request
		var req UpdateCategoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Find category
		var category models.DocumentCategory
		if err := db.First(&category, "id = ?", categoryUUID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Category not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Prevent editing "Unassigned" category names
		if category.NameCS == "Nepřiřazeno" || category.NameEN == "Unassigned" {
			if req.NameCS != nil || req.NameEN != nil {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Cannot rename the 'Unassigned' category",
				})
				return
			}
		}

		// Update fields
		updates := make(map[string]interface{})
		if req.NameCS != nil {
			updates["name_cs"] = *req.NameCS
		}
		if req.NameEN != nil {
			updates["name_en"] = *req.NameEN
		}
		if req.OrderIndex != nil {
			updates["order_index"] = *req.OrderIndex
		}

		if len(updates) > 0 {
			if err := db.Model(&category).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update category"})
				return
			}
		}

		// Fetch updated category
		if err := db.First(&category, "id = ?", categoryUUID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch updated category"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    category,
		})
	}
}

// DeleteCategory deletes a category and reassigns documents to Unassigned (admin only)
// DELETE /api/v1/admin/categories/:id
func DeleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		categoryID := c.Param("id")
		if categoryID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Category ID is required"})
			return
		}

		categoryUUID, err := uuid.Parse(categoryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid category ID"})
			return
		}

		// Find category
		var category models.DocumentCategory
		if err := db.First(&category, "id = ?", categoryUUID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Category not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Prevent deleting "Unassigned" category
		if category.NameCS == "Nepřiřazeno" || category.NameEN == "Unassigned" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Cannot delete the 'Unassigned' category",
			})
			return
		}

		// Find the "Unassigned" category for this subject and type
		var unassignedCategory models.DocumentCategory
		err = db.Where("subject_id = ? AND type = ? AND name_cs = ?",
			category.SubjectID, category.Type, "Nepřiřazeno").First(&unassignedCategory).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to find 'Unassigned' category for reassignment",
			})
			return
		}

		// Reassign all documents from this category to Unassigned
		if err := db.Model(&models.Document{}).
			Where("category_id = ?", categoryUUID).
			Update("category_id", unassignedCategory.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to reassign documents"})
			return
		}

		// Delete the category
		if err := db.Delete(&category).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete category"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Category deleted and documents reassigned to 'Unassigned'",
		})
	}
}

// ReorderCategories updates the order of multiple categories (admin only)
// PUT /api/v1/admin/categories/reorder
func ReorderCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ReorderCategoriesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Update each category's order_index
		for _, item := range req.Categories {
			categoryUUID, err := uuid.Parse(item.ID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Invalid category ID: " + item.ID,
				})
				return
			}

			if err := db.Model(&models.DocumentCategory{}).
				Where("id = ?", categoryUUID).
				Update("order_index", item.OrderIndex).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to update category order",
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Categories reordered successfully",
		})
	}
}

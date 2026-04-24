package handlers

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const MaxFileSize = 50 * 1024 * 1024 // 50 MB

var AllowedMimeTypes = map[string]bool{
	"application/pdf":                                true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true, // docx
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true, // pptx
	"application/vnd.ms-powerpoint": true, // ppt (legacy PowerPoint format)
	"image/jpeg":             true,
	"image/png":              true,
	"text/plain":             true,
	"text/csv":               true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true, // xlsx
}

func UploadDocument(db *gorm.DB, cfg *config.Config, storage *services.StorageService, tika *services.TextExtractionService, search *services.SearchService, activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if storage == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Storage service is not available. Please try again later."})
			return
		}

		subjectID := c.Param("id")
		userID := c.GetString("user_id")

		// Parse multipart form
		if err := c.Request.ParseMultipartForm(MaxFileSize); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File too large"})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file uploaded"})
			return
		}
		defer file.Close()

		// Get and validate type_id (DocumentType UUID)
		typeIDStr := c.Request.FormValue("type_id")
		if typeIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "type_id is required"})
			return
		}
		typeUUID, err := uuid.Parse(typeIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type_id format"})
			return
		}

		// Get and validate category_id
		categoryIDStr := c.Request.FormValue("category_id")
		var categoryID *uuid.UUID

		// Validate file size
		if header.Size > MaxFileSize {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File exceeds 50MB limit"})
			return
		}

		// Validate MIME type
		mimeType := header.Header.Get("Content-Type")
		if !AllowedMimeTypes[mimeType] {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Unsupported file type"})
			return
		}

		// Verify subject exists
		var subject models.Subject
		subjectUUID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid subject ID"})
			return
		}
		if err := db.First(&subject, "id = ?", subjectUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Subject not found"})
			return
		}

		// Verify document type exists and belongs to subject
		var docType models.DocumentType
		if err := db.Where("id = ? AND subject_id = ?", typeUUID, subjectUUID).First(&docType).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type for this subject"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
			}
			return
		}

		// Validate and assign category
		if categoryIDStr != "" {
			catUUID, err := uuid.Parse(categoryIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid category ID"})
				return
			}

			// Verify category exists, belongs to subject, and matches type
			var category models.DocumentCategory
			if err := db.Where("id = ? AND subject_id = ? AND type_id = ?", catUUID, subjectUUID, typeUUID).First(&category).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid category for this subject and type"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
				}
				return
			}
			categoryID = &catUUID
		} else {
			// Auto-assign to "Unassigned" category
			var unassignedCategory models.DocumentCategory
			if err := db.Where("subject_id = ? AND type_id = ? AND name_cs = ?", subjectUUID, typeUUID, "Nepřiřazeno").First(&unassignedCategory).Error; err != nil {
				// If no unassigned category exists, continue without category_id (will be NULL)
				categoryID = nil
			} else {
				categoryID = &unassignedCategory.ID
			}
		}

		// Generate unique filename
		ext := filepath.Ext(header.Filename)
		newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

		// Upload to storage
		if err := storage.UploadFile(file, newFilename, header.Size, mimeType); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to upload file"})
			return
		}

		// Extract text (best effort)
		var extractedText string
		if IsTextExtractable(mimeType) {
			text, err := tika.ExtractText(file, mimeType)
			if err == nil {
				extractedText = text
			}
		}

		// Create document record
		docID := uuid.New()
		userUUID, _ := uuid.Parse(userID)

		document := models.Document{
			ID:           docID,
			SubjectID:    subjectUUID,
			UploadedBy:   userUUID,
			TypeID:       &typeUUID,
			CategoryID:   categoryID,
			Filename:     newFilename,
			OriginalName: header.Filename,
			FileSize:     header.Size,
			MimeType:     mimeType,
			FilePath:     newFilename,
			ContentText:  extractedText,
		}

		if err := db.Create(&document).Error; err != nil {
			_ = storage.DeleteFile(newFilename)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save document record"})
			return
		}

		// no-op index call (tsvector maintained by DB)
		go func() {
			_ = search.IndexDocument(document)
		}()

		// Create activity
		go func() {
			if err := activity.CreateActivity(userUUID, models.ActivityDocumentUploaded, &subjectUUID, &docID, nil); err != nil {
				log.Printf("ERROR: Failed to create upload activity for document %s: %v", docID, err)
			}
		}()

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": document})
	}
}

func ListDocuments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		userIDStr := c.GetString("user_id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		var documents []models.Document
		
		// Join with favorites to get status and sort
		// Order by (user_favorite_documents.user_id IS NOT NULL) DESC
		query := db.Preload("Uploader").Preload("Category").
			Select("documents.*, (CASE WHEN ufd.user_id IS NOT NULL THEN true ELSE false END) as is_favorite").
			Joins("LEFT JOIN user_favorite_documents ufd ON documents.id = ufd.document_id AND ufd.user_id = ?", userIDStr).
			Where("documents.subject_id = ?", subjectID)

		query = query.Limit(limit).Offset(offset).Order("is_favorite DESC, documents.created_at desc")

		if err := query.Find(&documents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch documents"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": documents})
	}
}

func GetDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		docID := c.Param("id")
		var document models.Document
		if err := db.Preload("Uploader").Preload("Subject").First(&document, "id = ?", docID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": document})
	}
}

func DownloadDocument(db *gorm.DB, storage *services.StorageService, activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if storage == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Storage service is not available. Please try again later."})
			return
		}

		docID := c.Param("id")

		var document models.Document
		if err := db.First(&document, "id = ?", docID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document not found"})
			return
		}

		rc, fileSize, err := storage.DownloadFile(document.FilePath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "File not found in storage"})
			return
		}
		defer rc.Close()

		// Log download activity (async)
		go func() {
			userID := c.GetString("user_id")
			userUUID, err := uuid.Parse(userID)
			if err != nil {
				return
			}
			docUUID := document.ID
			subjectID := document.SubjectID
			if err := activity.CreateActivity(userUUID, models.ActivityDocumentDownloaded, &subjectID, &docUUID, nil); err != nil {
				log.Printf("ERROR: Failed to create download activity for document %s: %v", docUUID, err)
			}
		}()

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf("attachment; filename=\"%s\"", document.OriginalName),
		}

		c.DataFromReader(http.StatusOK, fileSize, document.MimeType, rc, extraHeaders)
	}
}

func DeleteDocument(db *gorm.DB, storage *services.StorageService, search *services.SearchService, activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if storage == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Storage service is not available. Please try again later."})
			return
		}

		docID := c.Param("id")
		userID := c.GetString("user_id")
		// Assume user role is available in context if we need to check admin
		// For now simple check: uploader or admin logic needs role.
		// `handlers/auth.go` puts `role` in claims, verify if middleware puts it in context.
		// Assuming middleware extracts claims to context usually.
		// If not, we might need to fetch user.

		var document models.Document
		if err := db.First(&document, "id = ?", docID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document not found"})
			return
		}

		// Check permissions
		// TODO: Better role check. For now, strict check on ID.
		if document.UploadedBy.String() != userID {
			// Fetch user to check if admin
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Not authorized to delete this document"})
					return
				}
			}
		}

		if err := storage.DeleteFile(document.FilePath); err != nil {
			log.Printf("Warning: Failed to delete file from storage: %v", err)
		}

		// Delete from Meilisearch
		go func() {
			if err := search.DeleteDocument(document.ID.String()); err != nil {
				log.Printf("ERROR: Failed to delete document %s from search index: %v", document.ID, err)
			}
		}()

		// Create activity
		go func() {
			userUUID, _ := uuid.Parse(userID)
			if err := activity.CreateActivity(userUUID, models.ActivityDocumentDeleted, &document.SubjectID, &document.ID, nil); err != nil {
				log.Printf("ERROR: Failed to create delete activity for document %s: %v", document.ID, err)
			}
		}()

		// Delete from DB
		if err := db.Delete(&document).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete document record"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Document deleted"})
	}
}

func Search(search *services.SearchService) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query parameter 'q' is required"})
			return
		}

		searchType := c.Query("type")        // "all", "documents", "subjects"
		subjectID := c.Query("subject_id")   // Filter by specific subject
		mimeType := c.Query("mime_type")     // Filter by file type (e.g., "application/pdf")
		exactMatch := c.Query("exact") == "true" // Exact match mode (disables fuzzy)

		result, err := search.SearchAll(query, searchType, subjectID, mimeType, exactMatch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Search failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
	}
}

func IsTextExtractable(mimeType string) bool {
	return mimeType == "application/pdf" ||
		mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
		mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
		mimeType == "application/vnd.ms-powerpoint" ||
		strings.HasPrefix(mimeType, "text/")
}

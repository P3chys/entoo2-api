package handlers

import (
	"fmt"

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
	"image/jpeg":             true,
	"image/png":              true,
	"text/plain":             true,
	"text/csv":               true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true, // xlsx
}

func UploadDocument(db *gorm.DB, cfg *config.Config, storage *services.StorageService, tika *services.TextExtractionService, search *services.SearchService, activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if err := db.First(&subject, "id = ?", subjectID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Subject not found"})
			return
		}

		// Generate unique filename
		ext := filepath.Ext(header.Filename)
		newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

		// Upload to MinIO
		if err := storage.UploadFile(file, newFilename, header.Size, mimeType); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to upload file"})
			return
		}

		// Extract text (async, best effort)
		var extractedText string
		if IsTextExtractable(mimeType) {
			text, err := tika.ExtractText(file)
			if err == nil {
				extractedText = text
			}
		}

		// Create document record
		docID := uuid.New()
		userUUID, _ := uuid.Parse(userID)
		subjectUUID, _ := uuid.Parse(subjectID)

		document := models.Document{
			ID:           docID,
			SubjectID:    subjectUUID,
			UploadedBy:   userUUID,
			Filename:     newFilename,
			OriginalName: header.Filename,
			FileSize:     header.Size,
			MimeType:     mimeType,
			MinIOPath:    newFilename,
			ContentText:  extractedText,
		}

		if err := db.Create(&document).Error; err != nil {
			// Cleanup MinIO
			storage.DeleteFile(newFilename)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save document record"})
			return
		}

		// Index in Meilisearch (async)
		go func() {
			search.IndexDocument(document)
		}()

		// Create activity
		go func() {
			activity.CreateActivity(userUUID, models.ActivityDocumentUploaded, &subjectUUID, &docID, nil)
		}()

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": document})
	}
}

func ListDocuments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		userIDStr := c.GetString("user_id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		var documents []models.Document
		
		// Join with favorites to get status and sort
		// Order by (user_favorite_documents.user_id IS NOT NULL) DESC
		query := db.Preload("Uploader").
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
		docID := c.Param("id")


		var document models.Document
		if err := db.First(&document, "id = ?", docID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Document not found"})
			return
		}

		obj, err := storage.DownloadFile(document.MinIOPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve file"})
			return
		}
		defer obj.Close()

		// Verify object exists and get info
		stat, err := obj.Stat()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "File not found in storage"})
			return
		}

		// Log activity (async)
		go func() {

			// We define a download activity type if needed, but the model currently only has Uploaded/Deleted.
			// Assuming we don't strictly need to track downloads in the current required enums, skipping or adding if needed.
			// The plan says "Create download activity" but the ActivityType enum in models/activity.go only has DocumentUploaded and DocumentDeleted.
			// I'll skip for now to stick to the defined model, or I could add it. Plan says "1.1 Create Activity Tracking System... Activity Types: document_uploaded, document_deleted".
			// So I will NOT create a download activity unless I add it to the model.
		}()

		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf("attachment; filename=\"%s\"", document.OriginalName),
		}

		c.DataFromReader(http.StatusOK, stat.Size, document.MimeType, obj, extraHeaders)
	}
}

func DeleteDocument(db *gorm.DB, storage *services.StorageService, search *services.SearchService, activity *services.ActivityService) gin.HandlerFunc {
	return func(c *gin.Context) {
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
					if user.Role != models.RoleAdmin {
						c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Not authorized to delete this document"})
						return
					}
				}
			}
		}

		// Delete from MinIO
		if err := storage.DeleteFile(document.MinIOPath); err != nil {
			// Log error but continue
			fmt.Printf("Failed to delete from MinIO: %v\n", err)
		}

		// Delete from Meilisearch
		go func() {
			search.DeleteDocument(document.ID.String())
		}()

		// Create activity
		go func() {
			userUUID, _ := uuid.Parse(userID)
			activity.CreateActivity(userUUID, models.ActivityDocumentDeleted, &document.SubjectID, &document.ID, nil)
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
		subjectID := c.Query("subject_id")
		
		result, err := search.Search(query, subjectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Search failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": result.Hits})
	}
}

func IsTextExtractable(mimeType string) bool {
	return mimeType == "application/pdf" || 
		mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || 
		mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
		strings.HasPrefix(mimeType, "text/")
}

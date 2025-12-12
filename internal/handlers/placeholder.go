package handlers

import (
	"net/http"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Placeholder handlers - to be implemented

func ListDocuments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
	}
}

func DownloadDocument(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
	}
}

func Search(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
	}
}


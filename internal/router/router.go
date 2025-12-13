package router

import (
	"log"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/handlers"
	"github.com/P3chys/entoo2-api/internal/middleware"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	// Initialize Services
	storageService, err := services.NewStorageService(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize storage service: %v", err)
	}

	tikaService := services.NewTextExtractionService(cfg)
	searchService := services.NewSearchService(cfg)
	activityService := services.NewActivityService(db)

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
	}))

	// Health check endpoint
	r.GET("/health", handlers.HealthCheck(db))

	// API v1 routes
	api := r.Group("/api/v1")
	{
		// Public routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register(db, cfg))
			auth.POST("/login", handlers.Login(db, cfg))
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthRequired(cfg))
		{
			// Auth
			protected.GET("/auth/me", handlers.GetCurrentUser(db))
			protected.POST("/auth/logout", handlers.Logout())

			// Semesters
			protected.GET("/semesters", handlers.ListSemesters(db))
			protected.GET("/semesters/:id", handlers.GetSemester(db))

			// Subjects
			protected.GET("/subjects", handlers.ListSubjects(db))
			protected.GET("/subjects/:id", handlers.GetSubject(db))

			// Documents
			protected.POST("/subjects/:id/documents", handlers.UploadDocument(db, cfg, storageService, tikaService, searchService, activityService))
			protected.GET("/subjects/:id/documents", handlers.ListDocuments(db))
			protected.GET("/documents/:id", handlers.GetDocument(db))
			protected.GET("/documents/:id/download", handlers.DownloadDocument(db, storageService, activityService))
			protected.DELETE("/documents/:id", handlers.DeleteDocument(db, storageService, searchService, activityService))

			// Activities
			protected.GET("/activities/recent", handlers.GetRecentActivities(activityService))

			// Search
			protected.GET("/search", handlers.Search(searchService))
		}

		// Admin routes
		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired(cfg), middleware.AdminRequired())
		{
			// Semester management
			admin.POST("/semesters", handlers.CreateSemester(db))
			admin.PUT("/semesters/:id", handlers.UpdateSemester(db))
			admin.DELETE("/semesters/:id", handlers.DeleteSemester(db))

			// Subject management
			admin.POST("/subjects", handlers.CreateSubject(db))
			admin.PUT("/subjects/:id", handlers.UpdateSubject(db))
			admin.DELETE("/subjects/:id", handlers.DeleteSubject(db))
		}
	}

	return r
}

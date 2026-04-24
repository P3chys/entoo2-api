package router

import (
	"log"
	"time"

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
	searchService := services.NewSearchService(db)
	activityService := services.NewActivityService(db)
	emailService := services.NewEmailService(cfg)

	statsService, err := services.NewStatsService(db, cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize stats service: %v. Stats will be unavailable.", err)
	} else {
		statsService.StartObservabilityLogger()
	}

	// Rate limiter (in-memory, no external dependency)
	rateLimiter := middleware.NewRateLimiter()

	// Analytics middleware (DB-backed)
	analytics := middleware.NewAnalytics(db)

	// Background cleanup of expired revoked tokens
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			db.Exec("DELETE FROM revoked_tokens WHERE expires_at < NOW()")
		}
	}()

	gin.SetMode(cfg.GinMode)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
	}))

	r.GET("/health", handlers.HealthCheck(db))

	api := r.Group("/api/v1")
	api.Use(analytics.TrackRequests())

	{
		// Public routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register(db, cfg, emailService))
			auth.POST("/login", handlers.Login(db, cfg))

			auth.GET("/verify-email/:token", rateLimiter.RateLimitByIP(10, 3600), handlers.VerifyEmail(db, cfg))
			auth.POST("/verify-email/request", rateLimiter.RateLimitByIP(5, 3600), handlers.RequestEmailVerification(db, cfg, emailService))

			auth.POST("/password-reset/request", rateLimiter.RateLimitByIP(3, 3600), handlers.RequestPasswordReset(db, cfg, emailService))
			auth.POST("/password-reset/confirm", rateLimiter.RateLimitByIP(5, 900), handlers.ResetPassword(db))
			auth.GET("/password-reset/verify/:token", rateLimiter.RateLimitByIP(10, 3600), handlers.VerifyResetToken(db))
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthRequired(cfg, db))
		{
			protected.GET("/auth/me", handlers.GetCurrentUser(db))
			protected.POST("/auth/logout", handlers.Logout(cfg, db))

			protected.GET("/semesters", handlers.ListSemesters(db))
			protected.GET("/semesters/:id", handlers.GetSemester(db))

			protected.GET("/subjects", handlers.ListSubjects(db))
			protected.GET("/subjects/:id", handlers.GetSubject(db))
			protected.POST("/subjects/:id/favorite", handlers.ToggleFavoriteSubject(db))

			protected.POST("/subjects/:id/documents", handlers.UploadDocument(db, cfg, storageService, tikaService, searchService, activityService))
			protected.GET("/subjects/:id/documents", handlers.ListDocuments(db))
			protected.POST("/documents/:id/favorite", handlers.ToggleFavoriteDocument(db))
			protected.GET("/documents/:id", handlers.GetDocument(db))
			protected.GET("/documents/:id/download", handlers.DownloadDocument(db, storageService, activityService))
			protected.DELETE("/documents/:id", handlers.DeleteDocument(db, storageService, searchService, activityService))

			protected.GET("/subjects/:id/types", handlers.ListDocumentTypes(db))
			protected.GET("/subjects/:id/categories", handlers.ListCategories(db))

			protected.POST("/subjects/:id/comments", handlers.CreateComment(db))
			protected.GET("/subjects/:id/comments", handlers.GetCommentsBySubject(db))
			protected.DELETE("/comments/:id", handlers.DeleteComment(db))

			protected.POST("/subjects/:id/questions", handlers.CreateQuestion(db))
			protected.GET("/subjects/:id/questions", handlers.GetQuestionsBySubject(db))
			protected.DELETE("/questions/:id", handlers.DeleteQuestion(db))
			protected.POST("/questions/:id/answers", handlers.CreateAnswer(db, cfg, storageService, tikaService, searchService))

			protected.GET("/activities/recent", handlers.GetRecentActivities(activityService))
			protected.GET("/favorites", handlers.ListFavorites(db))

			protected.POST("/teachers/:id/rate", handlers.RateTeacher(db))
			protected.DELETE("/teachers/:id/rate", handlers.DeleteTeacherRating(db))
			protected.GET("/teachers/:id/ratings", handlers.GetTeacherRatings(db))

			protected.GET("/search", handlers.Search(searchService))

			protected.GET("/subjects/:id/decks", handlers.ListDecksBySubject(db))
			protected.POST("/subjects/:id/decks", handlers.CreateDeck(db))
			protected.GET("/decks", handlers.ListPublicDecks(db))
			protected.GET("/decks/:id", handlers.GetDeck(db))
			protected.PUT("/decks/:id", handlers.UpdateDeck(db))
			protected.DELETE("/decks/:id", handlers.DeleteDeck(db))
			protected.POST("/decks/:id/favorite", handlers.ToggleFavoriteDeck(db))
			protected.GET("/decks/:id/progress", handlers.GetDeckProgress(db))
			protected.POST("/decks/:id/clone", handlers.CloneDeck(db))

			protected.POST("/decks/:id/cards", handlers.CreateFlashcard(db))
			protected.POST("/decks/:id/cards/bulk", handlers.BulkCreateFlashcards(db))
			protected.PUT("/cards/:id", handlers.UpdateFlashcard(db))
			protected.DELETE("/cards/:id", handlers.DeleteFlashcard(db))
			protected.PUT("/decks/:id/cards/reorder", handlers.ReorderFlashcards(db))

			protected.POST("/decks/:id/study/start", handlers.StartStudySession(db))
			protected.POST("/study-sessions/:id/review", handlers.ReviewFlashcard(db))
			protected.POST("/study-sessions/:id/complete", handlers.CompleteStudySession(db))
		}

		// Admin routes
		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired(cfg, db), middleware.AdminRequired())
		{
			admin.POST("/semesters", handlers.CreateSemester(db))
			admin.PUT("/semesters/:id", handlers.UpdateSemester(db))
			admin.DELETE("/semesters/:id", handlers.DeleteSemester(db))

			admin.POST("/subjects", handlers.CreateSubject(db))
			admin.PUT("/subjects/:id", handlers.UpdateSubject(db))
			admin.DELETE("/subjects/:id", handlers.DeleteSubject(db))

			admin.POST("/subjects/:id/types", handlers.CreateDocumentType(db))
			admin.PUT("/types/:id", handlers.UpdateDocumentType(db))
			admin.DELETE("/types/:id", handlers.DeleteDocumentType(db))
			admin.PUT("/types/reorder", handlers.ReorderDocumentTypes(db))

			admin.POST("/subjects/:id/categories", handlers.CreateCategory(db))
			admin.PUT("/categories/:id", handlers.UpdateCategory(db))
			admin.DELETE("/categories/:id", handlers.DeleteCategory(db))
			admin.PUT("/categories/reorder", handlers.ReorderCategories(db))

			if statsService != nil {
				admin.GET("/stats/overview", handlers.GetStatsOverview(statsService))
				admin.GET("/stats/users", handlers.GetUserStats(statsService))
				admin.GET("/stats/api", handlers.GetAPIStats(statsService))
				admin.GET("/stats/activity", handlers.GetActivityStats(statsService))
				admin.GET("/stats/system", handlers.GetSystemStats(statsService))
			}
		}
	}

	return r
}

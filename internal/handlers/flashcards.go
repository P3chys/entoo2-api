package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"log"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Request/Response types
type CreateDeckRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=200"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type UpdateDeckRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"is_public"`
}

type CreateFlashcardRequest struct {
	CardType      string   `json:"card_type"`                        // "standard" or "multiple_choice"
	FrontText     string   `json:"front_text" binding:"required"`
	BackText      string   `json:"back_text" binding:"required"`
	Options       []string `json:"options,omitempty"`                // Options for multiple choice
	CorrectOption *int     `json:"correct_option,omitempty"`         // Index of correct option (0-based)
	OrderIndex    *int     `json:"order_index"`
}

type BulkCreateFlashcardsRequest struct {
	Cards []CreateFlashcardRequest `json:"cards" binding:"required,min=1"`
}

type ReorderFlashcardsRequest struct {
	CardOrders []struct {
		ID         string `json:"id" binding:"required"`
		OrderIndex int    `json:"order_index"`
	} `json:"card_orders" binding:"required"`
}

// ListDecksBySubject returns all decks for a subject
func ListDecksBySubject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		userID := c.GetString("user_id")

		// Parse query parameter for including public decks
		includePublic := true
		if c.Query("include_public") == "false" {
			includePublic = false
		}

		// Validate subject ID
		subjectUUID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid subject ID",
				},
			})
			return
		}

		// Verify subject exists
		var subject models.Subject
		if err := db.First(&subject, "id = ?", subjectUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Subject not found",
				},
			})
			return
		}

		var decks []models.FlashcardDeck

		// Build query with favorite status and card count
		query := db.Preload("Creator").Preload("Subject").
			Select("flashcard_decks.*, "+
				"(SELECT COUNT(*) FROM flashcards WHERE flashcards.deck_id = flashcard_decks.id) as card_count, "+
				"(CASE WHEN ufd.user_id IS NOT NULL THEN true ELSE false END) as is_favorite").
			Joins("LEFT JOIN user_favorite_decks ufd ON flashcard_decks.id = ufd.flashcard_deck_id AND ufd.user_id = ?", userID).
			Where("flashcard_decks.subject_id = ?", subjectUUID)

		// Filter: user's own decks + public decks if includePublic
		if includePublic {
			query = query.Where("flashcard_decks.created_by = ? OR flashcard_decks.is_public = true", userID)
		} else {
			query = query.Where("flashcard_decks.created_by = ?", userID)
		}

		// Sort: Favorites first, then by created date
		query = query.Order("is_favorite DESC, flashcard_decks.created_at DESC")

		if err := query.Find(&decks).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch decks",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    decks,
		})
	}
}

// CreateDeck creates a new flashcard deck
func CreateDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID := c.Param("id")
		userID := c.GetString("user_id")

		var req CreateDeckRequest
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

		// Validate subject ID
		subjectUUID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid subject ID",
				},
			})
			return
		}

		// Verify subject exists
		var subject models.Subject
		if err := db.First(&subject, "id = ?", subjectUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Subject not found",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)

		// Create deck
		deck := models.FlashcardDeck{
			SubjectID:   subjectUUID,
			CreatedBy:   userUUID,
			Title:       req.Title,
			Description: req.Description,
			IsPublic:    req.IsPublic,
		}

		if err := db.Create(&deck).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create deck",
				},
			})
			return
		}

		// Reload with relations
		db.Preload("Creator").Preload("Subject").First(&deck, "id = ?", deck.ID)

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    deck,
		})
	}
}

// GetDeck returns a deck with all its flashcards
func GetDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck

		// Get deck with favorite status and card count
		err = db.Preload("Creator").
			Preload("Subject").
			Preload("Flashcards", func(db *gorm.DB) *gorm.DB {
				return db.Order("order_index ASC")
			}).
			Select("flashcard_decks.*, "+
				"(SELECT COUNT(*) FROM flashcards WHERE flashcards.deck_id = flashcard_decks.id) as card_count, "+
				"(CASE WHEN ufd.user_id IS NOT NULL THEN true ELSE false END) as is_favorite").
			Joins("LEFT JOIN user_favorite_decks ufd ON flashcard_decks.id = ufd.flashcard_deck_id AND ufd.user_id = ?", userID).
			Where("flashcard_decks.id = ?", deckUUID).
			First(&deck).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check access: owner, admin, or public
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID && !deck.IsPublic {
			// Check if user is admin
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to view this deck",
						},
					})
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    deck,
		})
	}
}

// UpdateDeck updates deck metadata
func UpdateDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		var req UpdateDeckRequest
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

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check ownership or admin
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to edit this deck",
						},
					})
					return
				}
			}
		}

		// Update fields
		if req.Title != nil {
			deck.Title = *req.Title
		}
		if req.Description != nil {
			deck.Description = *req.Description
		}
		if req.IsPublic != nil {
			deck.IsPublic = *req.IsPublic
		}

		if err := db.Save(&deck).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update deck",
				},
			})
			return
		}

		// Reload with relations
		db.Preload("Creator").Preload("Subject").First(&deck, "id = ?", deck.ID)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    deck,
		})
	}
}

// DeleteDeck deletes a flashcard deck
func DeleteDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check ownership or admin
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to delete this deck",
						},
					})
					return
				}
			}
		}

		// Delete deck (cascades to flashcards)
		if err := db.Delete(&deck).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete deck",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Deck deleted successfully",
		})
	}
}

// ToggleFavoriteDeck toggles favorite status for a deck
func ToggleFavoriteDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		// Verify deck exists
		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)

		// Check if already favorited
		var count int64
		db.Table("user_favorite_decks").
			Where("user_id = ? AND flashcard_deck_id = ?", userUUID, deckUUID).
			Count(&count)

		isFavorite := false
		if count > 0 {
			// Remove favorite
			db.Exec("DELETE FROM user_favorite_decks WHERE user_id = ? AND flashcard_deck_id = ?", userUUID, deckUUID)
			isFavorite = false
		} else {
			// Add favorite
			db.Exec("INSERT INTO user_favorite_decks (user_id, flashcard_deck_id) VALUES (?, ?)", userUUID, deckUUID)
			isFavorite = true
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"is_favorite": isFavorite,
		})
	}
}

// CreateFlashcard creates a single flashcard in a deck
func CreateFlashcard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		var req CreateFlashcardRequest
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

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check ownership or admin
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to add cards to this deck",
						},
					})
					return
				}
			}
		}

		// Determine order index
		orderIndex := 0
		if req.OrderIndex != nil {
			orderIndex = *req.OrderIndex
		} else {
			// Get max order index + 1
			var maxIndex int
			db.Model(&models.Flashcard{}).
				Where("deck_id = ?", deckUUID).
				Select("COALESCE(MAX(order_index), -1) + 1").
				Scan(&maxIndex)
			orderIndex = maxIndex
		}

		// Determine card type
		cardType := models.CardTypeStandard
		if req.CardType == "multiple_choice" {
			cardType = models.CardTypeMultipleChoice
		}

		// Create flashcard
		flashcard := models.Flashcard{
			DeckID:        deckUUID,
			CardType:      cardType,
			FrontText:     req.FrontText,
			BackText:      req.BackText,
			OrderIndex:    orderIndex,
			CorrectOption: -1, // Default for standard cards
		}

		// Handle multiple choice options
		if cardType == models.CardTypeMultipleChoice && len(req.Options) > 0 {
			optionsJSON, err := json.Marshal(req.Options)
			if err == nil {
				flashcard.Options = string(optionsJSON)
			}
			if req.CorrectOption != nil {
				flashcard.CorrectOption = *req.CorrectOption
			}
		}

		if err := db.Create(&flashcard).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create flashcard",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    flashcard,
		})
	}
}

// BulkCreateFlashcards creates multiple flashcards at once
func BulkCreateFlashcards(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		var req BulkCreateFlashcardsRequest
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

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check ownership or admin
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to add cards to this deck",
						},
					})
					return
				}
			}
		}

		// Get starting order index
		var maxIndex int
		db.Model(&models.Flashcard{}).
			Where("deck_id = ?", deckUUID).
			Select("COALESCE(MAX(order_index), -1) + 1").
			Scan(&maxIndex)

		// Create flashcards
		flashcards := make([]models.Flashcard, len(req.Cards))
		for i, cardReq := range req.Cards {
			cardType := models.CardTypeStandard
			if cardReq.CardType == "multiple_choice" {
				cardType = models.CardTypeMultipleChoice
			}

			flashcards[i] = models.Flashcard{
				DeckID:        deckUUID,
				CardType:      cardType,
				FrontText:     cardReq.FrontText,
				BackText:      cardReq.BackText,
				OrderIndex:    maxIndex + i,
				CorrectOption: -1,
			}

			// Handle multiple choice options
			if cardType == models.CardTypeMultipleChoice && len(cardReq.Options) > 0 {
				optionsJSON, err := json.Marshal(cardReq.Options)
				if err == nil {
					flashcards[i].Options = string(optionsJSON)
				}
				if cardReq.CorrectOption != nil {
					flashcards[i].CorrectOption = *cardReq.CorrectOption
				}
			}
		}

		if err := db.Create(&flashcards).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create flashcards",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    flashcards,
		})
	}
}

// UpdateFlashcard updates a flashcard
func UpdateFlashcard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cardID := c.Param("id")
		userID := c.GetString("user_id")

		var req struct {
			CardType      *string  `json:"card_type"`
			FrontText     *string  `json:"front_text"`
			BackText      *string  `json:"back_text"`
			Options       []string `json:"options"`
			CorrectOption *int     `json:"correct_option"`
		}
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

		cardUUID, err := uuid.Parse(cardID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid card ID",
				},
			})
			return
		}

		var flashcard models.Flashcard
		if err := db.First(&flashcard, "id = ?", cardUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Flashcard not found",
				},
			})
			return
		}

		// Get deck and check ownership
		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", flashcard.DeckID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to verify permissions",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to edit this flashcard",
						},
					})
					return
				}
			}
		}

		// Update fields
		if req.CardType != nil {
			if *req.CardType == "multiple_choice" {
				flashcard.CardType = models.CardTypeMultipleChoice
			} else {
				flashcard.CardType = models.CardTypeStandard
			}
		}
		if req.FrontText != nil {
			flashcard.FrontText = *req.FrontText
		}
		if req.BackText != nil {
			flashcard.BackText = *req.BackText
		}
		if len(req.Options) > 0 {
			optionsJSON, err := json.Marshal(req.Options)
			if err == nil {
				flashcard.Options = string(optionsJSON)
			}
		}
		if req.CorrectOption != nil {
			flashcard.CorrectOption = *req.CorrectOption
		}

		if err := db.Save(&flashcard).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update flashcard",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    flashcard,
		})
	}
}

// DeleteFlashcard deletes a flashcard
func DeleteFlashcard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cardID := c.Param("id")
		userID := c.GetString("user_id")

		cardUUID, err := uuid.Parse(cardID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid card ID",
				},
			})
			return
		}

		var flashcard models.Flashcard
		if err := db.First(&flashcard, "id = ?", cardUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Flashcard not found",
				},
			})
			return
		}

		// Get deck and check ownership
		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", flashcard.DeckID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to verify permissions",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to delete this flashcard",
						},
					})
					return
				}
			}
		}

		if err := db.Delete(&flashcard).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete flashcard",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Flashcard deleted successfully",
		})
	}
}

// ReorderFlashcards updates the order of flashcards in a deck
func ReorderFlashcards(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		var req ReorderFlashcardsRequest
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

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check ownership or admin
		userUUID, _ := uuid.Parse(userID)
		if deck.CreatedBy != userUUID {
			var user models.User
			if err := db.First(&user, "id = ?", userID).Error; err == nil {
				if user.Role != models.RoleAdmin {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "FORBIDDEN",
							"message": "You do not have permission to reorder cards in this deck",
						},
					})
					return
				}
			}
		}

		// Update order indexes in a transaction
		err = db.Transaction(func(tx *gorm.DB) error {
			for _, cardOrder := range req.CardOrders {
				cardUUID, err := uuid.Parse(cardOrder.ID)
				if err != nil {
					return err
				}

				if err := tx.Model(&models.Flashcard{}).
					Where("id = ? AND deck_id = ?", cardUUID, deckUUID).
					Update("order_index", cardOrder.OrderIndex).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to reorder flashcards",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Flashcards reordered successfully",
		})
	}
}

// StartStudySession creates a new study session and returns cards to study
func StartStudySession(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		log.Printf("[DEBUG] StartStudySession called - userID: %s", userID)
		if userID == "" {
			log.Printf("[ERROR] StartStudySession - userID is empty!")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "User not authenticated",
				},
			})
			return
		}

		deckID := c.Param("id")
		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		userUUID, err := uuid.Parse(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_USER_ID",
					"message": "Invalid user ID",
				},
			})
			return
		}

		// Get deck with flashcards
		var deck models.FlashcardDeck
		if err := db.Preload("Flashcards", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_index ASC")
		}).First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		if len(deck.Flashcards) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NO_CARDS",
					"message": "This deck has no flashcards to study",
				},
			})
			return
		}

		// Create study session
		session := models.FlashcardStudySession{
			ID:          uuid.New(),
			UserID:      userUUID,
			DeckID:      deckUUID,
			StartedAt:   time.Now(),
			CardsStudied: 0,
			CardsCorrect: 0,
		}

		if err := db.Create(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create study session",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"session_id":  session.ID,
				"cards":       deck.Flashcards,
				"cards_total": len(deck.Flashcards),
			},
		})
	}
}

// ReviewFlashcard records a flashcard review during a study session
func ReviewFlashcard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "User not authenticated",
				},
			})
			return
		}

		sessionID := c.Param("id")
		sessionUUID, err := uuid.Parse(sessionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid session ID",
				},
			})
			return
		}

		var input struct {
			FlashcardID string `json:"flashcard_id" binding:"required"`
			Quality     int    `json:"quality" binding:"min=0,max=5"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_INPUT",
					"message": err.Error(),
				},
			})
			return
		}

		flashcardUUID, err := uuid.Parse(input.FlashcardID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid flashcard ID",
				},
			})
			return
		}

		userUUID, err := uuid.Parse(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_USER_ID",
					"message": "Invalid user ID",
				},
			})
			return
		}

		// Verify session belongs to user
		var session models.FlashcardStudySession
		if err := db.First(&session, "id = ? AND user_id = ?", sessionUUID, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Study session not found",
				},
			})
			return
		}

		// Create review record
		review := models.FlashcardReview{
			ID:          uuid.New(),
			SessionID:   sessionUUID,
			FlashcardID: flashcardUUID,
			UserID:      userUUID,
			Quality:     input.Quality,
			CreatedAt:   time.Now(),
		}

		if err := db.Create(&review).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to record review",
				},
			})
			return
		}

		// Update or create user progress using SM-2 algorithm
		var progress models.UserFlashcardProgress
		err = db.Where("user_id = ? AND flashcard_id = ?", userUUID, flashcardUUID).First(&progress).Error

		srService := services.NewSpacedRepetitionService()

		isNewProgress := err == gorm.ErrRecordNotFound
		if isNewProgress {
			// Create new progress record
			progress = models.UserFlashcardProgress{
				ID:          uuid.New(),
				UserID:      userUUID,
				FlashcardID: flashcardUUID,
				DeckID:      session.DeckID,
				EaseFactor:  2.5,
				Interval:    0,
				Repetitions: 0,
			}
		}

		// Calculate next review using SM-2
		result := srService.CalculateNextReview(&progress, input.Quality)
		progress.EaseFactor = result.EaseFactor
		progress.Interval = result.Interval
		progress.Repetitions = result.Repetitions
		progress.NextReviewDate = &result.NextReviewDate
		now := time.Now()
		progress.LastReviewedAt = &now
		progress.TotalReviews++
		if input.Quality >= 3 {
			progress.CorrectReviews++
		}

		// Use Create for new records, Save for existing ones
		if isNewProgress {
			if err := db.Create(&progress).Error; err != nil {
				log.Printf("[WARN] Failed to create progress: %v", err)
			}
		} else {
			if err := db.Save(&progress).Error; err != nil {
				log.Printf("[WARN] Failed to save progress: %v", err)
			}
		}

		// Update session stats
		session.CardsStudied++
		if input.Quality >= 3 {
			session.CardsCorrect++
		}

		if err := db.Save(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update session stats",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"cards_studied":    session.CardsStudied,
				"cards_correct":    session.CardsCorrect,
				"next_review_date": result.NextReviewDate,
				"ease_factor":      result.EaseFactor,
				"interval":         result.Interval,
			},
		})
	}
}

// CompleteStudySession marks a study session as completed
func CompleteStudySession(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "User not authenticated",
				},
			})
			return
		}

		sessionID := c.Param("id")
		sessionUUID, err := uuid.Parse(sessionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid session ID",
				},
			})
			return
		}

		var input struct {
			DurationSeconds int `json:"duration_seconds"`
		}

		if err := c.ShouldBindJSON(&input); err == nil && input.DurationSeconds > 0 {
			// Optional: use provided duration
		}

		// Verify session belongs to user
		var session models.FlashcardStudySession
		if err := db.First(&session, "id = ? AND user_id = ?", sessionUUID, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Study session not found",
				},
			})
			return
		}

		// Mark as completed
		now := time.Now()
		session.CompletedAt = &now
		
		if input.DurationSeconds > 0 {
			session.DurationSeconds = input.DurationSeconds
		} else {
			// Calculate duration based on start time
			session.DurationSeconds = int(now.Sub(session.StartedAt).Seconds())
		}

		if err := db.Save(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to complete session",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"session_id":        session.ID,
				"cards_studied":     session.CardsStudied,
				"cards_correct":     session.CardsCorrect,
				"duration_seconds":  session.DurationSeconds,
				"accuracy":          float64(session.CardsCorrect) / float64(session.CardsStudied) * 100,
			},
		})
	}
}

// GetDeckProgress returns progress statistics for a deck
func GetDeckProgress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)

		// Verify deck exists
		var deck models.FlashcardDeck
		if err := db.First(&deck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Get total cards
		var totalCards int64
		db.Model(&models.Flashcard{}).Where("deck_id = ?", deckUUID).Count(&totalCards)

		// Get user progress records
		var progressRecords []models.UserFlashcardProgress
		db.Where("user_id = ? AND deck_id = ?", userUUID, deckUUID).Find(&progressRecords)

		// Calculate statistics using SM-2 service
		srService := services.NewSpacedRepetitionService()
		progress := srService.CalculateDeckProgress(int(totalCards), progressRecords)

		// Get total study time
		var totalStudyTime int64
		db.Model(&models.FlashcardStudySession{}).
			Where("user_id = ? AND deck_id = ? AND completed_at IS NOT NULL", userUUID, deckUUID).
			Select("COALESCE(SUM(duration_seconds), 0)").
			Scan(&totalStudyTime)
		progress.TotalStudyTimeSeconds = int(totalStudyTime)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    progress,
		})
	}
}

// CloneDeck creates a copy of a public deck for the current user
func CloneDeck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deckID := c.Param("id")
		userID := c.GetString("user_id")

		deckUUID, err := uuid.Parse(deckID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid deck ID",
				},
			})
			return
		}

		userUUID, _ := uuid.Parse(userID)

		// Get original deck with flashcards
		var originalDeck models.FlashcardDeck
		if err := db.Preload("Flashcards").First(&originalDeck, "id = ?", deckUUID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Deck not found",
				},
			})
			return
		}

		// Check if deck is public or user owns it
		if !originalDeck.IsPublic && originalDeck.CreatedBy != userUUID {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Cannot clone a private deck",
				},
			})
			return
		}

		// Create new deck as a copy
		newDeck := models.FlashcardDeck{
			SubjectID:   originalDeck.SubjectID,
			CreatedBy:   userUUID,
			Title:       originalDeck.Title + " (Copy)",
			Description: originalDeck.Description,
			IsPublic:    false, // New copies are private by default
		}

		if err := db.Create(&newDeck).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create deck copy",
				},
			})
			return
		}

		// Clone all flashcards
		if len(originalDeck.Flashcards) > 0 {
			newCards := make([]models.Flashcard, len(originalDeck.Flashcards))
			for i, card := range originalDeck.Flashcards {
				newCards[i] = models.Flashcard{
					DeckID:        newDeck.ID,
					CardType:      card.CardType,
					FrontText:     card.FrontText,
					BackText:      card.BackText,
					Options:       card.Options,
					CorrectOption: card.CorrectOption,
					OrderIndex:    card.OrderIndex,
				}
			}

			if err := db.Create(&newCards).Error; err != nil {
				// Rollback: delete the deck we just created
				db.Delete(&newDeck)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to clone flashcards",
					},
				})
				return
			}
		}

		// Reload deck with relations
		db.Preload("Creator").Preload("Subject").First(&newDeck, "id = ?", newDeck.ID)

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    newDeck,
		})
	}
}

// ListPublicDecks returns all public decks, optionally filtered by subject
func ListPublicDecks(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		subjectID := c.Query("subject_id")

		query := db.Preload("Creator").Preload("Subject").
			Select("flashcard_decks.*, "+
				"(SELECT COUNT(*) FROM flashcards WHERE flashcards.deck_id = flashcard_decks.id) as card_count, "+
				"(CASE WHEN ufd.user_id IS NOT NULL THEN true ELSE false END) as is_favorite").
			Joins("LEFT JOIN user_favorite_decks ufd ON flashcard_decks.id = ufd.flashcard_deck_id AND ufd.user_id = ?", userID).
			Where("flashcard_decks.is_public = true")

		if subjectID != "" {
			subjectUUID, err := uuid.Parse(subjectID)
			if err == nil {
				query = query.Where("flashcard_decks.subject_id = ?", subjectUUID)
			}
		}

		query = query.Order("flashcard_decks.created_at DESC")

		var decks []models.FlashcardDeck
		if err := query.Find(&decks).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to fetch decks",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    decks,
		})
	}
}

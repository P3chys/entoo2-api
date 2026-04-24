package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/P3chys/entoo2-api/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/P3chys/entoo2-api/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
	Language    string `json:"language"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User         *models.User `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

func Register(db *gorm.DB, cfg *config.Config, emailService *services.EmailService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
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

		// Check if user exists
		var existingUser models.User
		if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "E-mail již existuje",
				},
			})
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se hashovat heslo",
				},
			})
			return
		}

		// Generate verification token
		plainToken, err := utils.GenerateSecureToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vygenerovat ověřovací token",
				},
			})
			return
		}

		hashedToken, err := utils.HashToken(plainToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se hashovat ověřovací token",
				},
			})
			return
		}

		// Create user with email verification required
		now := time.Now()
		user := models.User{
			Email:                   req.Email,
			PasswordHash:            string(hashedPassword),
			DisplayName:             req.DisplayName,
			Language:                req.Language,
			Role:                    models.RoleStudent,
			EmailVerified:           false,
			EmailVerificationToken:  &hashedToken,
			EmailVerificationSentAt: &now,
		}

		if user.Language == "" {
			user.Language = "cs"
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vytvořit uživatele",
				},
			})
			return
		}

		// Send verification email asynchronously so registration doesn't block
		go func() {
			if err := emailService.SendVerificationEmail(user.Email, plainToken, user.Language); err != nil {
				log.Printf("Failed to send verification email to %s: %v", user.Email, err)
			}
		}()

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data": gin.H{
				"message":    "Registrace úspěšná. Zkontrolujte prosím svůj e-mail pro ověření účtu.",
				"email_sent": true,
			},
		})
	}
}

func Login(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
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

		// Find user
		var user models.User
		if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Neplatné přihlašovací údaje",
				},
			})
			return
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Neplatné přihlašovací údaje",
				},
			})
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "EMAIL_NOT_VERIFIED",
					"message": "Prosím ověřte svou e-mailovou adresu před přihlášením.",
				},
			})
			return
		}

		// Generate tokens
		accessToken, err := generateToken(user.ID, user.Role, cfg.JWTSecret, cfg.JWTAccessExpiry)
		if err != nil {
			log.Printf("Failed to generate access token for user %s: %v", user.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vygenerovat přístupový token",
				},
			})
			return
		}

		refreshToken, err := generateToken(user.ID, user.Role, cfg.JWTSecret, cfg.JWTRefreshExpiry)
		if err != nil {
			log.Printf("Failed to generate refresh token for user %s: %v", user.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vygenerovat obnovovací token",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": AuthResponse{
				User:         &user,
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			},
		})
	}
}

func GetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")

		var user models.User
		if err := db.First(&user, "id = ?", userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Uživatel nenalezen",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    user,
		})
	}
}

func Logout(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tokenString == "" {
			c.JSON(http.StatusNoContent, nil)
			return
		}

		// Parse token to get expiry, then insert into revoked_tokens
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err == nil && token.Valid {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if exp, ok := claims["exp"].(float64); ok {
					expiresAt := time.Unix(int64(exp), 0)
					if time.Until(expiresAt) > 0 {
						jti := middleware.TokenHash(tokenString)
						db.Exec(
							"INSERT INTO revoked_tokens (jti, expires_at) VALUES (?, ?) ON CONFLICT DO NOTHING",
							jti, expiresAt,
						)
					}
				}
			}
		}

		c.JSON(http.StatusNoContent, nil)
	}
}

// RequestEmailVerificationRequest is the request body for requesting email verification
type RequestEmailVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestEmailVerification resends the email verification link
func RequestEmailVerification(db *gorm.DB, cfg *config.Config, emailService *services.EmailService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RequestEmailVerificationRequest
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

		// Find user
		var user models.User
		if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			// Don't reveal if email exists
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "Pokud účet s tímto e-mailem existuje, byl odeslán ověřovací e-mail.",
				},
			})
			return
		}

		// Check if already verified
		if user.EmailVerified {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "E-mail je již ověřen.",
				},
			})
			return
		}

		// Generate new verification token
		plainToken, err := utils.GenerateSecureToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vygenerovat ověřovací token",
				},
			})
			return
		}

		hashedToken, err := utils.HashToken(plainToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se hashovat ověřovací token",
				},
			})
			return
		}

		// Update user with new token
		now := time.Now()
		user.EmailVerificationToken = &hashedToken
		user.EmailVerificationSentAt = &now

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se aktualizovat uživatele",
				},
			})
			return
		}

		// Send verification email asynchronously
		go func() {
			if err := emailService.SendVerificationEmail(user.Email, plainToken, user.Language); err != nil {
				log.Printf("Failed to send verification email to %s: %v", user.Email, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Ověřovací e-mail odeslán. Zkontrolujte prosím svou schránku.",
			},
		})
	}
}

// VerifyEmail verifies a user's email address using the token from the email
func VerifyEmail(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Token je povinný",
				},
			})
			return
		}

		// Hash the plain token and query directly by hash
		hashedToken, err := utils.HashToken(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Chyba při zpracování tokenu",
				},
			})
			return
		}

		var matchedUser models.User
		if err := db.Where("email_verification_token = ?", hashedToken).First(&matchedUser).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Neplatný nebo expirovaný ověřovací token",
				},
			})
			return
		}

		// Check token expiry (24 hours)
		expiry, err := time.ParseDuration(cfg.EmailVerificationExpiry)
		if err != nil {
			log.Printf("Invalid email verification expiry duration '%s': %v, using default 24h", cfg.EmailVerificationExpiry, err)
			expiry = 24 * time.Hour
		}
		if matchedUser.EmailVerificationSentAt != nil {
			if time.Since(*matchedUser.EmailVerificationSentAt) > expiry {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Ověřovací token vypršel. Požádejte prosím o nový.",
					},
				})
				return
			}
		}

		// Mark email as verified
		now := time.Now()
		matchedUser.EmailVerified = true
		matchedUser.EmailVerifiedAt = &now
		matchedUser.EmailVerificationToken = nil
		matchedUser.EmailVerificationSentAt = nil

		if err := db.Save(&matchedUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se ověřit e-mail",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "E-mail úspěšně ověřen. Nyní se můžete přihlásit.",
			},
		})
	}
}

// RequestPasswordResetRequest is the request body for password reset
type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestPasswordReset sends a password reset email to the user
func RequestPasswordReset(db *gorm.DB, cfg *config.Config, emailService *services.EmailService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RequestPasswordResetRequest
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

		// Find user
		var user models.User
		if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			// Don't reveal if email exists
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "Pokud účet s tímto e-mailem existuje, byl odeslán e-mail pro obnovení hesla.",
				},
			})
			return
		}

		// Generate reset token
		plainToken, err := utils.GenerateSecureToken(32)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se vygenerovat token pro obnovení",
				},
			})
			return
		}

		hashedToken, err := utils.HashToken(plainToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se hashovat token pro obnovení",
				},
			})
			return
		}

		// Set reset token and expiry
		now := time.Now()
		expiry, err := time.ParseDuration(cfg.PasswordResetExpiry)
		if err != nil {
			log.Printf("Invalid password reset expiry duration '%s': %v, using default 1h", cfg.PasswordResetExpiry, err)
			expiry = 1 * time.Hour
		}
		expiresAt := now.Add(expiry)

		user.PasswordResetToken = &hashedToken
		user.PasswordResetSentAt = &now
		user.PasswordResetExpiresAt = &expiresAt

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se aktualizovat uživatele",
				},
			})
			return
		}

		// Send password reset email asynchronously
		go func() {
			if err := emailService.SendPasswordResetEmail(user.Email, plainToken, user.Language); err != nil {
				log.Printf("Failed to send password reset email to %s: %v", user.Email, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Pokud účet s tímto e-mailem existuje, byl odeslán e-mail pro obnovení hesla.",
			},
		})
	}
}

// VerifyResetToken checks if a password reset token is valid
func VerifyResetToken(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Token je povinný",
				},
			})
			return
		}

		// Hash the plain token and query directly
		hashedToken, err := utils.HashToken(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Chyba při zpracování tokenu",
				},
			})
			return
		}

		var matchedUser models.User
		if err := db.Where("password_reset_token = ?", hashedToken).First(&matchedUser).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Neplatný nebo expirovaný token pro obnovení",
				},
			})
			return
		}

		// Check token expiry
		if matchedUser.PasswordResetExpiresAt != nil {
			if time.Now().After(*matchedUser.PasswordResetExpiresAt) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Token pro obnovení vypršel. Požádejte prosím o nový.",
					},
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Token je platný",
				"valid":   true,
			},
		})
	}
}

// ResetPasswordRequest is the request body for resetting password
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ResetPassword resets a user's password using the reset token
func ResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ResetPasswordRequest
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

		// Hash the plain token and query directly
		hashedToken, err := utils.HashToken(req.Token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Chyba při zpracování tokenu",
				},
			})
			return
		}

		var matchedUser models.User
		if err := db.Where("password_reset_token = ?", hashedToken).First(&matchedUser).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Neplatný nebo expirovaný token pro obnovení",
				},
			})
			return
		}

		// Check token expiry
		if matchedUser.PasswordResetExpiresAt != nil {
			if time.Now().After(*matchedUser.PasswordResetExpiresAt) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Token pro obnovení vypršel. Požádejte prosím o nový.",
					},
				})
				return
			}
		}

		// Hash new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se hashovat heslo",
				},
			})
			return
		}

		// Update password and clear reset token
		matchedUser.PasswordHash = string(hashedPassword)
		matchedUser.PasswordResetToken = nil
		matchedUser.PasswordResetSentAt = nil
		matchedUser.PasswordResetExpiresAt = nil

		if err := db.Save(&matchedUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Nepodařilo se obnovit heslo",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Heslo úspěšně obnoveno. Nyní se můžete přihlásit s novým heslem.",
			},
		})
	}
}

func generateToken(userID uuid.UUID, role models.UserRole, secret string, expiry string) (string, error) {
	duration, err := time.ParseDuration(expiry)
	if err != nil {
		duration = 15 * time.Minute
	}

	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"role":    role,
		"exp":     time.Now().Add(duration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

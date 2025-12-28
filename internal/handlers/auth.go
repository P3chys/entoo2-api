package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/P3chys/entoo2-api/internal/services"
	"github.com/P3chys/entoo2-api/internal/utils"
	"github.com/gin-gonic/gin"
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
					"message": "Email already exists",
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
					"message": "Failed to hash password",
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
					"message": "Failed to generate verification token",
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
					"message": "Failed to hash verification token",
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
					"message": "Failed to create user",
				},
			})
			return
		}

		// Send verification email
		if err := emailService.SendVerificationEmail(user.Email, plainToken, user.Language); err != nil {
			log.Printf("Failed to send verification email to %s: %v", user.Email, err)
			// Don't fail registration if email fails - user can request resend
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data": gin.H{
				"message":    "Registration successful. Please check your email to verify your account.",
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
					"message": "Invalid credentials",
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
					"message": "Invalid credentials",
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
					"message": "Please verify your email address before logging in.",
				},
			})
			return
		}

		// Generate tokens
		accessToken, _ := generateToken(user.ID, user.Role, cfg.JWTSecret, cfg.JWTAccessExpiry)
		refreshToken, _ := generateToken(user.ID, user.Role, cfg.JWTSecret, cfg.JWTRefreshExpiry)

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
					"message": "User not found",
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

func Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a full implementation, we would add the token to a blacklist in Redis
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
					"message": "If an account with this email exists, a verification email has been sent.",
				},
			})
			return
		}

		// Check if already verified
		if user.EmailVerified {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "Email is already verified.",
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
					"message": "Failed to generate verification token",
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
					"message": "Failed to hash verification token",
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
					"message": "Failed to update user",
				},
			})
			return
		}

		// Send verification email
		if err := emailService.SendVerificationEmail(user.Email, plainToken, user.Language); err != nil {
			log.Printf("Failed to send verification email to %s: %v", user.Email, err)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Verification email sent. Please check your inbox.",
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
					"message": "Token is required",
				},
			})
			return
		}

		// Find user with verification token
		var users []models.User
		if err := db.Where("email_verification_token IS NOT NULL").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Database error",
				},
			})
			return
		}

		// Find matching user by verifying token hash
		var matchedUser *models.User
		for i := range users {
			if users[i].EmailVerificationToken != nil {
				if utils.VerifyToken(*users[i].EmailVerificationToken, token) {
					matchedUser = &users[i]
					break
				}
			}
		}

		if matchedUser == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid or expired verification token",
				},
			})
			return
		}

		// Check token expiry (24 hours)
		expiry, _ := time.ParseDuration(cfg.EmailVerificationExpiry)
		if matchedUser.EmailVerificationSentAt != nil {
			if time.Since(*matchedUser.EmailVerificationSentAt) > expiry {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Verification token has expired. Please request a new one.",
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

		if err := db.Save(matchedUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to verify email",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Email verified successfully. You can now log in.",
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
					"message": "If an account with this email exists, a password reset email has been sent.",
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
					"message": "Failed to generate reset token",
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
					"message": "Failed to hash reset token",
				},
			})
			return
		}

		// Set reset token and expiry
		now := time.Now()
		expiry, _ := time.ParseDuration(cfg.PasswordResetExpiry)
		expiresAt := now.Add(expiry)

		user.PasswordResetToken = &hashedToken
		user.PasswordResetSentAt = &now
		user.PasswordResetExpiresAt = &expiresAt

		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update user",
				},
			})
			return
		}

		// Send password reset email
		if err := emailService.SendPasswordResetEmail(user.Email, plainToken, user.Language); err != nil {
			log.Printf("Failed to send password reset email to %s: %v", user.Email, err)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "If an account with this email exists, a password reset email has been sent.",
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
					"message": "Token is required",
				},
			})
			return
		}

		// Find user with reset token
		var users []models.User
		if err := db.Where("password_reset_token IS NOT NULL").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Database error",
				},
			})
			return
		}

		// Find matching user
		var matchedUser *models.User
		for i := range users {
			if users[i].PasswordResetToken != nil {
				if utils.VerifyToken(*users[i].PasswordResetToken, token) {
					matchedUser = &users[i]
					break
				}
			}
		}

		if matchedUser == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid or expired reset token",
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
						"message": "Reset token has expired. Please request a new one.",
					},
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Token is valid",
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

		// Find user with reset token
		var users []models.User
		if err := db.Where("password_reset_token IS NOT NULL").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Database error",
				},
			})
			return
		}

		// Find matching user
		var matchedUser *models.User
		for i := range users {
			if users[i].PasswordResetToken != nil {
				if utils.VerifyToken(*users[i].PasswordResetToken, req.Token) {
					matchedUser = &users[i]
					break
				}
			}
		}

		if matchedUser == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid or expired reset token",
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
						"message": "Reset token has expired. Please request a new one.",
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
					"message": "Failed to hash password",
				},
			})
			return
		}

		// Update password and clear reset token
		matchedUser.PasswordHash = string(hashedPassword)
		matchedUser.PasswordResetToken = nil
		matchedUser.PasswordResetSentAt = nil
		matchedUser.PasswordResetExpiresAt = nil

		if err := db.Save(matchedUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to reset password",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"message": "Password reset successfully. You can now log in with your new password.",
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

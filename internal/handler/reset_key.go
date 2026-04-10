package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type ResetKeyHandler struct {
	userRepo repository.UserRepository
}

func NewResetKeyHandler(userRepo repository.UserRepository) *ResetKeyHandler {
	return &ResetKeyHandler{userRepo: userRepo}
}

// ResetKeyRequest godoc
// @Summary 请求重置 API Key
// @Description 通过用户名请求重置 API Key（要求用户已绑定 GitHub）
// @Tags auth
// @Accept json
// @Produce json
// @Param body body object true "请求体 {username: string}"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /auth/reset-key/request [post]
func (h *ResetKeyHandler) ResetKeyRequest(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Look up user by username
	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			middleware.RespondError(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to look up user")
		return
	}

	// Build the GitHub OAuth URL for reset-key flow
	oauthURL := "/api/v1/auth/github?state=/reset-key/" + req.Username

	// Check if user has GitHub bound
	if user.GitHubID == nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":             "GITHUB_REQUIRED",
			"message":           "请先绑定 GitHub 账号才能重置 API Key",
			"github_oauth_url":  oauthURL,
		})
		return
	}

	// Already bound — return the OAuth URL for the user to verify identity
	c.JSON(http.StatusOK, gin.H{
		"github_oauth_url": oauthURL,
	})
}

// HandleResetKeyCallback is called from the GitHub OAuth callback when the state
// indicates a reset-key flow. It generates a reset token and stores it in the DB.
func (h *ResetKeyHandler) HandleResetKeyCallback(ctx context.Context, user *model.User) (string, error) {
	resetToken := uuid.New().String()
	expires := time.Now().Add(24 * time.Hour)

	if err := h.userRepo.UpdateResetToken(ctx, user.ID, resetToken, expires); err != nil {
		return "", err
	}

	return resetToken, nil
}

// ResetKeyConfirm godoc
// @Summary 确认重置 API Key
// @Description 使用重置令牌生成新的 API Key
// @Tags auth
// @Accept json
// @Produce json
// @Param body body object true "请求体 {reset_token: string}"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Failure 410 {object} middleware.ErrorResponse
// @Router /auth/reset-key/confirm [post]
func (h *ResetKeyHandler) ResetKeyConfirm(c *gin.Context) {
	var req struct {
		ResetToken string `json:"reset_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Look up user by reset token
	user, err := h.userRepo.GetUserByResetToken(c.Request.Context(), req.ResetToken)
	if err != nil {
		if err == repository.ErrUserNotFound {
			middleware.RespondError(c, http.StatusNotFound, "INVALID_TOKEN", "Reset token not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to look up reset token")
		return
	}

	// Verify token hasn't expired
	if user.APIKeyResetExpires == nil || time.Now().After(*user.APIKeyResetExpires) {
		middleware.RespondError(c, http.StatusGone, "TOKEN_EXPIRED", "Reset token has expired")
		return
	}

	// Generate new API key (same logic as Register: two UUIDs concatenated, first 32 chars)
	newAPIKey := uuid.New().String() + uuid.New().String()
	newAPIKey = newAPIKey[:32]

	// Update the API key
	if err := h.userRepo.UpdateAPIKey(c.Request.Context(), user.ID, newAPIKey); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update API key")
		return
	}

	// Clear the reset token
	if err := h.userRepo.ClearResetToken(c.Request.Context(), user.ID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to clear reset token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key": newAPIKey,
	})
}

// GitHubStatus godoc
// @Summary 获取当前用户的 GitHub 绑定状态
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object
// @Router /users/me/github-status [get]
func (h *ResetKeyHandler) GitHubStatus(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "User not found")
		return
	}

	bound := user.GitHubID != nil
	var githubUsername string
	if user.GitHubUsername != nil {
		githubUsername = *user.GitHubUsername
	}

	c.JSON(http.StatusOK, gin.H{
		"bound":           bound,
		"github_username": githubUsername,
	})
}

// CheckGitHubByUsername godoc
// @Summary 检查用户名是否已绑定 GitHub（无需认证）
// @Tags auth
// @Produce json
// @Param username path string true "用户名"
// @Success 200 {object} object
// @Router /auth/check-github/{username} [get]
func (h *ResetKeyHandler) CheckGitHubByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Username is required")
		return
	}

	user, err := h.userRepo.GetByUsername(c.Request.Context(), username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			middleware.RespondError(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to look up user")
		return
	}

	bound := user.GitHubID != nil
	var githubID int64
	if user.GitHubID != nil {
		githubID = *user.GitHubID
	}

	c.JSON(http.StatusOK, gin.H{
		"bound":     bound,
		"github_id": githubID,
	})
}

// ResetKeyDirect godoc
// @Summary 直接重置 API Key
// @Description 已认证用户直接生成新的 API Key
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object
// @Failure 500 {object} middleware.ErrorResponse
// @Router /auth/reset-key [post]
func (h *ResetKeyHandler) ResetKeyDirect(c *gin.Context) {
	userIDStr := c.GetString(middleware.ContextUserID)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "INVALID_USER_ID"})
		return
	}

	newKey := uuid.New().String() + uuid.New().String()
	newKey = newKey[:32]

	if err := h.userRepo.UpdateAPIKey(c.Request.Context(), userID, newKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "INTERNAL_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key": newKey,
	})
}

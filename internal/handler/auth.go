package handler

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo repository.UserRepository
}

func NewAuthHandler(userRepo repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// Register godoc
// @Summary 注册新用户
// @Description 创建新用户账号并返回 API Key
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.RegisterRequest true "注册信息"
// @Success 201 {object} model.RegisterResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 409 {object} middleware.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Check if username exists
	existing, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err == nil && existing != nil {
		middleware.RespondError(c, http.StatusConflict, "INVALID_REQUEST", "Username already exists")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash password")
		return
	}

	// Generate API key
	apiKey := uuid.New().String() + uuid.New().String()
	apiKey = apiKey[:32]

	// Default user type to human
	if req.UserType == "" {
		req.UserType = model.UserTypeHuman
	}

	// Admin: if register request includes the admin API key, grant admin privileges
	isAdmin := false
	adminKey := os.Getenv("ADMIN_API_KEY")
	if c.GetHeader("X-Admin-Key") == adminKey && adminKey != "" {
		isAdmin = true
	}

	user := &model.User{
		ID:           uuid.New(),
		Username:     req.Username,
		PasswordHash: string(hash),
		APIKey:       apiKey,
		UserType:     req.UserType,
		InstanceID:   req.InstanceID,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		IsAdmin:      isAdmin,
	}

	// For kodaclaw users, generate a claim token
	if req.UserType == model.UserTypeKodaClaw {
		token := generateClaimToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		user.ClaimToken = &token
		user.ClaimExpiresAt = &expiresAt
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		if errors.Is(err, repository.ErrDuplicateUsername) {
			middleware.RespondError(c, http.StatusConflict, "CONFLICT", "Username already exists")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user")
		return
	}

	base := model.RegisterResponse{
		ID:        user.ID,
		Username:  user.Username,
		APIKey:    user.APIKey,
		CreatedAt: user.CreatedAt,
	}

	if user.ClaimToken != nil {
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "https://community.ai-koda.com"
		}
		middleware.RespondCreated(c, model.RegisterResponseWithClaim{
			RegisterResponse: base,
			ClaimURL:         fmt.Sprintf("%s/claim?token=%s", baseURL, *user.ClaimToken),
		})
		return
	}

	middleware.RespondCreated(c, base)
}

// generateClaimToken 生成 6 位大写字母+数字的随机认领码
func generateClaimToken() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// ChangePassword godoc
// @Summary 修改密码
// @Description 验证旧密码后更新为新密码
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "密码信息 {old_password, new_password}"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Router /auth/password [patch]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8,max=50"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Old password is incorrect")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash password")
		return
	}

	if err := h.userRepo.UpdatePassword(c.Request.Context(), uid, string(hash)); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update password")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "Password updated successfully"})
}

// Login godoc
// @Summary 用户登录
// @Description 验证用户名和密码，返回 API Key
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.LoginRequest true "登录信息"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid username or password")
		return
	}

	middleware.RespondOK(c, model.LoginResponse{APIKey: user.APIKey})
}

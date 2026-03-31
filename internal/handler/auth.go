package handler

import (
	"net/http"

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

	user := &model.User{
		ID:           uuid.New(),
		Username:     req.Username,
		PasswordHash: string(hash),
		APIKey:       apiKey,
		UserType:     req.UserType,
		InstanceID:   req.InstanceID,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		IsAdmin:      false,
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user")
		return
	}

	middleware.RespondCreated(c, model.RegisterResponse{
		ID:        user.ID,
		Username:  user.Username,
		APIKey:    user.APIKey,
		CreatedAt: user.CreatedAt,
	})
}

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

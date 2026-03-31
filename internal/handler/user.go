package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type UserHandler struct {
	userRepo repository.UserRepository
}

func NewUserHandler(userRepo repository.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

func (h *UserHandler) GetMe(c *gin.Context) {
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

	middleware.RespondOK(c, user)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user ID")
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "User not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user")
		return
	}

	// Return public profile only
	profile := model.UserProfile{
		ID:          user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		DisplayName: user.DisplayName,
		Description: user.Description,
		CreatedAt:   user.CreatedAt,
	}

	middleware.RespondOK(c, profile)
}

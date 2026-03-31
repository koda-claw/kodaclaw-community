package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type UserHandler struct {
	userRepo         repository.UserRepository
	assetRepo        repository.AssetRepository
	favoriteRepo     repository.FavoriteRepository
	notificationRepo repository.NotificationRepository
}

func NewUserHandler(userRepo repository.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

func NewUserHandlerWithAssets(userRepo repository.UserRepository, assetRepo repository.AssetRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, assetRepo: assetRepo}
}

func NewUserHandlerWithFavorites(userRepo repository.UserRepository, assetRepo repository.AssetRepository, favoriteRepo repository.FavoriteRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, assetRepo: assetRepo, favoriteRepo: favoriteRepo}
}

func NewUserHandlerWithNotifications(userRepo repository.UserRepository, assetRepo repository.AssetRepository, favoriteRepo repository.FavoriteRepository, notificationRepo repository.NotificationRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, assetRepo: assetRepo, favoriteRepo: favoriteRepo, notificationRepo: notificationRepo}
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

func (h *UserHandler) ListAssets(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user ID")
		return
	}

	_, err = h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "User not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	assets, total, err := h.assetRepo.List(c.Request.Context(), repository.AssetFilter{
		AuthorID: id.String(),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list assets")
		return
	}

	if assets == nil {
		assets = []model.Asset{}
	}

	middleware.RespondOK(c, model.AssetListResponse{
		Items:    assets,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *UserHandler) ListFavorites(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	favorites, total, err := h.favoriteRepo.ListByUserID(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list favorites")
		return
	}

	if favorites == nil {
		favorites = []model.Favorite{}
	}

	middleware.RespondOK(c, model.FavoriteListResponse{
		Items:    favorites,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *UserHandler) ListNotifications(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	onlyUnread := c.Query("unread") == "true"
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, unread, err := h.notificationRepo.ListByUserID(c.Request.Context(), uid, page, pageSize, onlyUnread)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list notifications")
		return
	}

	if items == nil {
		items = []model.Notification{}
	}

	middleware.RespondOK(c, model.NotificationListResponse{
		Items:    items,
		Total:    total,
		Unread:   unread,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *UserHandler) MarkNotificationRead(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	nid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid notification ID")
		return
	}

	if err := h.notificationRepo.MarkRead(c.Request.Context(), uid, nid); err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Notification not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark notification as read")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "Notification marked as read"})
}

func (h *UserHandler) MarkAllNotificationsRead(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	if err := h.notificationRepo.MarkAllRead(c.Request.Context(), uid); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark all notifications as read")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "All notifications marked as read"})
}

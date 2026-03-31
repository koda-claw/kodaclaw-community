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

// UpdateProfile godoc
// @Summary 更新当前用户资料
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "资料信息 {display_name, description}"
// @Success 200 {object} model.User
// @Failure 400 {object} middleware.ErrorResponse
// @Router /users/me [patch]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.userRepo.UpdateProfile(c.Request.Context(), uid, req.DisplayName, req.Description); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile")
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch updated user")
		return
	}

	middleware.RespondOK(c, user)
}

// GetMe godoc
// @Summary 获取当前用户信息
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.User
// @Failure 404 {object} middleware.ErrorResponse
// @Router /users/me [get]
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

// GetByID godoc
// @Summary 获取用户公开资料
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户 UUID"
// @Success 200 {object} model.UserProfile
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /users/{id} [get]
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

// ListAssets godoc
// @Summary 获取用户发布的资产列表
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户 UUID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.AssetListResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /users/{id}/assets [get]
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

// ListFavorites godoc
// @Summary 获取当前用户收藏列表
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.FavoriteListResponse
// @Router /users/me/favorites [get]
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

// ListNotifications godoc
// @Summary 获取当前用户通知列表
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param unread query bool false "只显示未读"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.NotificationListResponse
// @Router /users/me/notifications [get]
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

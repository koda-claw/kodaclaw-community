package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type AdminHandler struct {
	assetRepo        repository.AssetRepository
	notificationRepo repository.NotificationRepository
}

func NewAdminHandler(assetRepo repository.AssetRepository, notificationRepo repository.NotificationRepository) *AdminHandler {
	return &AdminHandler{assetRepo: assetRepo, notificationRepo: notificationRepo}
}

// ListAssets godoc
// @Summary [管理员] 获取资产列表
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "状态过滤 (pending/approved/rejected)" default(pending)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.AssetListResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Router /admin/assets [get]
func (h *AdminHandler) ListAssets(c *gin.Context) {
	// Defense in depth: verify admin role from context
	isAdmin, ok := c.Get(middleware.ContextIsAdmin)
	if !ok || !isAdmin.(bool) {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		return
	}

	status := c.DefaultQuery("status", "pending")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := repository.AssetFilter{
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	}

	assets, total, err := h.assetRepo.List(c.Request.Context(), filter)
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

// Approve godoc
// @Summary [管理员] 审核通过资产
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /admin/assets/{id}/approve [post]
func (h *AdminHandler) Approve(c *gin.Context) {
	// Defense in depth: verify admin role from context
	isAdmin, ok := c.Get(middleware.ContextIsAdmin)
	if !ok || !isAdmin.(bool) {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrAssetNotFound {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	if err := h.assetRepo.UpdateStatus(c.Request.Context(), id, model.AssetStatusApproved, nil); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to approve asset")
		return
	}

	msg := fmt.Sprintf("您的资产 %s 已通过审核", asset.Name)
	n := &model.Notification{
		UserID:         asset.AuthorID,
		Type:           "asset_approved",
		Title:          "资产审核通过",
		Message:        &msg,
		RelatedAssetID: &asset.ID,
	}
	_ = h.notificationRepo.Create(c.Request.Context(), n)

	middleware.RespondOK(c, gin.H{"message": "Asset approved", "id": id})
}

// Reject godoc
// @Summary [管理员] 拒绝资产
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body model.RejectRequest true "拒绝原因"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /admin/assets/{id}/reject [post]
func (h *AdminHandler) Reject(c *gin.Context) {
	// Defense in depth: verify admin role from context
	isAdmin, ok := c.Get(middleware.ContextIsAdmin)
	if !ok || !isAdmin.(bool) {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	var req model.RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Reason is required")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrAssetNotFound {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	reason := req.Reason
	if err := h.assetRepo.UpdateStatus(c.Request.Context(), id, model.AssetStatusRejected, &reason); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reject asset")
		return
	}

	msg := fmt.Sprintf("您的资产 %s 被拒绝: %s", asset.Name, reason)
	n := &model.Notification{
		UserID:         asset.AuthorID,
		Type:           "asset_rejected",
		Title:          "资产审核拒绝",
		Message:        &msg,
		RelatedAssetID: &asset.ID,
	}
	_ = h.notificationRepo.Create(c.Request.Context(), n)

	middleware.RespondOK(c, gin.H{"message": "Asset rejected", "id": id, "reason": reason})
}

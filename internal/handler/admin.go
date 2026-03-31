package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type AdminHandler struct {
	assetRepo        repository.AssetRepository
	notificationRepo repository.NotificationRepository
	versionRepo      repository.AssetVersionRepository
	storagePath      string
}

func NewAdminHandler(assetRepo repository.AssetRepository, notificationRepo repository.NotificationRepository, versionRepo repository.AssetVersionRepository, storagePath string) *AdminHandler {
	return &AdminHandler{
		assetRepo:        assetRepo,
		notificationRepo: notificationRepo,
		versionRepo:      versionRepo,
		storagePath:      storagePath,
	}
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

// CleanupOrphans 清理磁盘上孤立的资产目录（无对应 asset_versions 记录）
func (h *AdminHandler) CleanupOrphans(c *gin.Context) {
	isAdmin, ok := c.Get(middleware.ContextIsAdmin)
	if !ok || !isAdmin.(bool) {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		return
	}

	ctx := c.Request.Context()

	// 获取所有有效的 file_key（来自数据库）
	fileKeys, err := h.versionRepo.ListAllFileKeys(ctx)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list file keys")
		return
	}

	// 构建有效版本目录集合：{assetID}/{version}
	validDirs := make(map[string]struct{}, len(fileKeys))
	for _, key := range fileKeys {
		// file_key 格式：{assetID}/{version}/{filename}
		parts := strings.SplitN(key, string(filepath.Separator), 3)
		if len(parts) < 2 {
			// 兼容斜杠分隔
			parts = strings.SplitN(key, "/", 3)
		}
		if len(parts) >= 2 {
			validDirs[parts[0]+"/"+parts[1]] = struct{}{}
		}
	}

	// 扫描存储目录
	assetEntries, err := os.ReadDir(h.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			middleware.RespondOK(c, gin.H{"deleted": 0, "message": "Storage directory does not exist"})
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read storage directory")
		return
	}

	var deleted []string
	var deleteErrors []string

	for _, assetEntry := range assetEntries {
		if !assetEntry.IsDir() {
			continue
		}
		assetDir := filepath.Join(h.storagePath, assetEntry.Name())
		versionEntries, err := os.ReadDir(assetDir)
		if err != nil {
			continue
		}

		assetHasValid := false
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}
			key := assetEntry.Name() + "/" + versionEntry.Name()
			if _, ok := validDirs[key]; ok {
				assetHasValid = true
				continue
			}
			// 孤立版本目录
			orphanPath := filepath.Join(assetDir, versionEntry.Name())
			if err := os.RemoveAll(orphanPath); err != nil {
				deleteErrors = append(deleteErrors, fmt.Sprintf("failed to remove %s: %v", orphanPath, err))
			} else {
				deleted = append(deleted, key)
			}
		}

		// 如果资产目录下已无有效版本，删除整个资产目录
		if !assetHasValid {
			// 重新检查是否还有内容
			remaining, _ := os.ReadDir(assetDir)
			if len(remaining) == 0 {
				if err := os.RemoveAll(assetDir); err != nil {
					deleteErrors = append(deleteErrors, fmt.Sprintf("failed to remove asset dir %s: %v", assetDir, err))
				}
			}
		}
	}

	middleware.RespondOK(c, gin.H{
		"deleted":       len(deleted),
		"deleted_paths": deleted,
		"errors":        deleteErrors,
	})
}

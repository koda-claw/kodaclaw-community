package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type AdminHandler struct {
	assetRepo repository.AssetRepository
}

func NewAdminHandler(assetRepo repository.AssetRepository) *AdminHandler {
	return &AdminHandler{assetRepo: assetRepo}
}

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

	// Check asset exists
	if _, err := h.assetRepo.GetByID(c.Request.Context(), id); err != nil {
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

	middleware.RespondOK(c, gin.H{"message": "Asset approved", "id": id})
}

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

	// Check asset exists
	if _, err := h.assetRepo.GetByID(c.Request.Context(), id); err != nil {
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

	middleware.RespondOK(c, gin.H{"message": "Asset rejected", "id": id, "reason": reason})
}

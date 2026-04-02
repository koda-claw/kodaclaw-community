package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"github.com/vanzheng/kodaclaw-community/internal/service"
)

type ReviewHandler struct {
	reviewRepo  repository.ReviewRepository
	assetRepo   repository.AssetRepository
	activitySvc *service.ActivityService
}

func NewReviewHandler(reviewRepo repository.ReviewRepository, assetRepo repository.AssetRepository, activitySvc *service.ActivityService) *ReviewHandler {
	return &ReviewHandler{reviewRepo: reviewRepo, assetRepo: assetRepo, activitySvc: activitySvc}
}

// Create godoc
// @Summary 创建资产评论
// @Tags reviews
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body model.CreateReviewRequest true "评论内容"
// @Success 201 {object} model.Review
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Failure 409 {object} middleware.ErrorResponse
// @Router /assets/{id}/reviews [post]
func (h *ReviewHandler) Create(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	// Check asset exists and is approved
	asset, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
	if err != nil || asset.Status != model.AssetStatusApproved {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
		return
	}

	var req model.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user ID")
		return
	}

	review := &model.Review{
		AssetID:       assetID,
		UserID:        uid,
		Content:       req.Content,
		Compatibility: req.Compatibility,
		Usefulness:    req.Usefulness,
		Security:      req.Security,
	}

	// Upsert: update if exists, create if not
	existing, err := h.reviewRepo.GetByUserAndAsset(c.Request.Context(), assetID, uid)
	if err == nil && existing != nil {
		// Update existing review
		review.ID = existing.ID
		if err := h.reviewRepo.Update(c.Request.Context(), review); err != nil {
			middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update review")
			return
		}
	} else {
		// Create new review
		review.ID = uuid.New()
		if err := h.reviewRepo.Create(c.Request.Context(), review); err != nil {
			middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review")
			return
		}
	}

	go h.assetRepo.UpdateAvgRating(context.Background(), assetID)

	if h.activitySvc != nil {
		h.activitySvc.Record(c.Request.Context(), uid, "rate", &assetID)
	}

	middleware.RespondOK(c, review)
}

// List godoc
// @Summary 获取资产评论列表
// @Tags reviews
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.ReviewListResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Router /assets/{id}/reviews [get]
func (h *ReviewHandler) List(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
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

	reviews, total, err := h.reviewRepo.ListByAssetID(c.Request.Context(), assetID, page, pageSize)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list reviews")
		return
	}

	if reviews == nil {
		reviews = []model.Review{}
	}

	middleware.RespondOK(c, model.ReviewListResponse{
		Items:    reviews,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

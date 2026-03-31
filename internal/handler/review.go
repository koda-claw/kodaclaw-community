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

type ReviewHandler struct {
	reviewRepo repository.ReviewRepository
	assetRepo  repository.AssetRepository
}

func NewReviewHandler(reviewRepo repository.ReviewRepository, assetRepo repository.AssetRepository) *ReviewHandler {
	return &ReviewHandler{reviewRepo: reviewRepo, assetRepo: assetRepo}
}

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
	uid, _ := uuid.Parse(userID)

	review := &model.Review{
		ID:            uuid.New(),
		AssetID:       assetID,
		UserID:        uid,
		Content:       req.Content,
		Compatibility: req.Compatibility,
		Usefulness:    req.Usefulness,
		Security:      req.Security,
	}

	if err := h.reviewRepo.Create(c.Request.Context(), review); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review")
		return
	}

	// Fetch user name for response
	middleware.RespondCreated(c, review)
}

func (h *ReviewHandler) List(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

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

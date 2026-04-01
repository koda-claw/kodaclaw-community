package handler

import (
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

//go:embed bootstrap-skill.md
var bootstrapSkillContent string


type PublicHandler struct {
	assetRepo   repository.AssetRepository
	versionRepo repository.AssetVersionRepository
	reviewRepo  repository.ReviewRepository
	userRepo    repository.UserRepository
	storagePath string
}

func NewPublicHandler(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, reviewRepo repository.ReviewRepository, userRepo repository.UserRepository, storagePath string) *PublicHandler {
	return &PublicHandler{assetRepo: assetRepo, versionRepo: versionRepo, reviewRepo: reviewRepo, userRepo: userRepo, storagePath: storagePath}
}

// ListSkills godoc
// GET /api/v1/public/skills
func (h *PublicHandler) ListSkills(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	sort := c.DefaultQuery("sort", "created_at")
	if sort != "downloads" && sort != "created_at" && sort != "rating" {
		sort = "created_at"
	}

	filter := repository.AssetFilter{
		Type:     c.Query("type"),
		Tag:      c.Query("tag"),
		Query:    c.Query("q"),
		Sort:     sort,
		Page:     page,
		PageSize: pageSize,
	}

	assets, total, err := h.assetRepo.List(c.Request.Context(), filter)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list skills")
		return
	}

	if assets == nil {
		assets = []model.Asset{}
	}

	// Strip large fields from list response
	for i := range assets {
		assets[i].Readme = nil
		assets[i].SkillContent = nil
	}

	middleware.RespondOK(c, model.AssetListResponse{
		Items:    assets,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetSkill godoc
// GET /api/v1/public/skills/:name
func (h *PublicHandler) GetSkill(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}
	middleware.RespondOK(c, asset)
}

// GetSkillContent godoc
// GET /api/v1/public/skills/:name/SKILL.md
func (h *PublicHandler) GetSkillContent(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}
	if asset.SkillContent == nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "SKILL.md not available")
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(*asset.SkillContent))
}

// DownloadSkill godoc
// GET /api/v1/public/skills/:name/download
func (h *PublicHandler) DownloadSkill(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}

	av, err := h.versionRepo.GetCurrent(c.Request.Context(), asset.ID)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset version not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	filePath := filepath.Join(h.storagePath, av.FileKey)
	c.FileAttachment(filePath, fmt.Sprintf("%s-%s.zip", name, av.Version))
}

// BootstrapSkill godoc
// GET /skill.md
func (h *PublicHandler) BootstrapSkill(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(bootstrapSkillContent))
}

// ListReviews godoc
// GET /api/v1/public/reviews/:id
func (h *PublicHandler) ListReviews(c *gin.Context) {
	assetID, err := parseUUID(c, "id")
	if err != nil {
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
	middleware.RespondOK(c, gin.H{"reviews": reviews, "total": total})
}

// DownloadSkillByID godoc
// GET /api/v1/public/skills/download/:id
func (h *PublicHandler) DownloadSkillByID(c *gin.Context) {
	assetID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}

	av, err := h.versionRepo.GetCurrent(c.Request.Context(), asset.ID)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset version not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	filePath := filepath.Join(h.storagePath, av.FileKey)
	c.FileAttachment(filePath, fmt.Sprintf("%s-%s.zip", asset.Name, av.Version))
}
// Stats returns community statistics (public)
// GET /api/v1/public/stats
func (h *PublicHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Use existing List to get approved asset count (page_size=1, filter approved)
	_, assetCount, err := h.assetRepo.List(ctx, repository.AssetFilter{PageSize: 1})
	if err != nil {
		assetCount = 0
	}
	
	// Get total users
	userCount, err := h.userRepo.Count(ctx)
	if err != nil {
		userCount = 0
	}
	
	// Get total downloads (sum of install_count)
	downloadCount, err := h.assetRepo.TotalDownloads(ctx)
	if err != nil {
		downloadCount = 0
	}
	
	middleware.RespondOK(c, gin.H{
		"assets":   assetCount,
		"users":    userCount,
		"downloads": downloadCount,
	})
}

func parseUUID(c *gin.Context, param string) (uuid.UUID, error) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid UUID")
		return uuid.Nil, err
	}
	return id, nil
}

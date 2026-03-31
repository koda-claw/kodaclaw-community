package handler

import (
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

//go:embed bootstrap-skill.md
var bootstrapSkillContent string


type PublicHandler struct {
	assetRepo   repository.AssetRepository
	versionRepo repository.AssetVersionRepository
	storagePath string
}

func NewPublicHandler(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, storagePath string) *PublicHandler {
	return &PublicHandler{assetRepo: assetRepo, versionRepo: versionRepo, storagePath: storagePath}
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

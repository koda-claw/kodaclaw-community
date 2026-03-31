package handler

import (
	"fmt"
	"io"
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

type AssetHandler struct {
	assetRepo    repository.AssetRepository
	versionRepo  repository.AssetVersionRepository
	userRepo     repository.UserRepository
	storagePath  string
}

func NewAssetHandler(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, userRepo repository.UserRepository, storagePath string) *AssetHandler {
	return &AssetHandler{
		assetRepo:   assetRepo,
		versionRepo: versionRepo,
		userRepo:    userRepo,
		storagePath: storagePath,
	}
}

func (h *AssetHandler) Create(c *gin.Context) {
	var req model.UploadAssetRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "File is required")
		return
	}
	defer file.Close()

	// Validate zip extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Only .zip files are allowed")
		return
	}

	assetID := uuid.New()

	// Create storage directory
	dir := filepath.Join(h.storagePath, assetID.String(), req.Version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create storage directory")
		return
	}

	// Save file
	filePath := filepath.Join(dir, header.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save file")
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save file")
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	uid, _ := uuid.Parse(userID)

	// Parse tags
	var tags []string
	if req.Tags != "" {
		tags = strings.Split(req.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// Create asset record
	asset := &model.Asset{
		ID:           assetID,
		Name:         req.Name,
		Type:         req.Type,
		Description:  req.Description,
		AuthorID:     uid,
		Status:       model.AssetStatusPending,
		Tags:         tags,
	}

	if err := h.assetRepo.Create(c.Request.Context(), asset); err != nil {
		os.RemoveAll(filepath.Join(h.storagePath, assetID.String()))
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create asset")
		return
	}

	// Create version record
	var changelog *string
	if req.Changelog != "" {
		changelog = &req.Changelog
	}
	fileKey := filepath.Join(assetID.String(), req.Version, header.Filename)
	av := &model.AssetVersion{
		ID:        uuid.New(),
		AssetID:   assetID,
		Version:   req.Version,
		FileKey:   fileKey,
		FileSize:  written,
		Changelog: changelog,
	}
	if err := h.versionRepo.Create(c.Request.Context(), av); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create version")
		return
	}

	// Set current version
	h.assetRepo.UpdateCurrentVersion(c.Request.Context(), assetID, req.Version)

	// Fetch created asset with author name
	created, _ := h.assetRepo.GetByID(c.Request.Context(), assetID)
	middleware.RespondCreated(c, created)
}

func (h *AssetHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.AssetFilter{
		Type:     c.Query("type"),
		Tag:      c.Query("tag"),
		Query:    c.Query("q"),
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

func (h *AssetHandler) GetByID(c *gin.Context) {
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

	// Only show approved assets, or own assets
	userID := c.GetString(middleware.ContextUserID)
	if asset.Status != model.AssetStatusApproved && asset.AuthorID.String() != userID {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
		return
	}

	middleware.RespondOK(c, asset)
}

func (h *AssetHandler) Download(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	version := c.Query("version")
	var av *model.AssetVersion
	if version != "" {
		av, err = h.versionRepo.GetByVersion(c.Request.Context(), id, version)
	} else {
		av, err = h.versionRepo.GetCurrent(c.Request.Context(), id)
	}
	if err != nil {
		if err == repository.ErrAssetNotFound {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset version not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	filePath := filepath.Join(h.storagePath, av.FileKey)
	c.FileAttachment(filePath, fmt.Sprintf("%s-%s.zip", id, av.Version))
}

func (h *AssetHandler) ListVersions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	versions, err := h.versionRepo.ListByAssetID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list versions")
		return
	}

	if versions == nil {
		versions = []model.AssetVersion{}
	}
	middleware.RespondOK(c, versions)
}

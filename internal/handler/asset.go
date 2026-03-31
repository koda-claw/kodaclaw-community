package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

const maxFileSize = 50 * 1024 * 1024 // 50MB

var versionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

type AssetHandler struct {
	assetRepo    repository.AssetRepository
	versionRepo  repository.AssetVersionRepository
	userRepo     repository.UserRepository
	favoriteRepo repository.FavoriteRepository
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

func NewAssetHandlerWithFavorites(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, userRepo repository.UserRepository, favoriteRepo repository.FavoriteRepository, storagePath string) *AssetHandler {
	return &AssetHandler{
		assetRepo:    assetRepo,
		versionRepo:  versionRepo,
		userRepo:     userRepo,
		favoriteRepo: favoriteRepo,
		storagePath:  storagePath,
	}
}

func (h *AssetHandler) Create(c *gin.Context) {
	var req model.UploadAssetRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Validate version format
	if !versionRegex.MatchString(req.Version) {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Version must be in format x.y.z (e.g. 1.0.0)")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "File is required")
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > maxFileSize {
		middleware.RespondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "File size exceeds 50MB limit")
		return
	}

	// Validate zip extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Only .zip files are allowed")
		return
	}

	// Check ZIP magic bytes (PK signature)
	buf := make([]byte, 2)
	n, _ := file.Read(buf)
	if n < 2 || buf[0] != 0x50 || buf[1] != 0x4B {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_FILE", "File is not a valid ZIP archive")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process file")
		return
	}

	// Sanitize filename to prevent path traversal
	safeFilename := filepath.Base(header.Filename)

	assetID := uuid.New()

	// Create storage directory
	dir := filepath.Join(h.storagePath, assetID.String(), req.Version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create storage directory")
		return
	}

	// Save file
	filePath := filepath.Join(dir, safeFilename)
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
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user ID")
		return
	}

	// Parse tags
	var tags []string
	if req.Tags != "" {
		tags = strings.Split(req.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		// Validate tags
		if len(tags) > 10 {
			middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Maximum 10 tags allowed")
			return
		}
		for _, tag := range tags {
			if len(tag) > 30 {
				middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Tag too long (max 30 chars)")
				return
			}
			matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-]+$`, tag)
			if !matched {
				middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Tag must only contain letters, numbers, and hyphens")
				return
			}
		}
	}

	// Create asset record
	asset := &model.Asset{
		ID:          assetID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		AuthorID:    uid,
		Status:      model.AssetStatusPending,
		Tags:        tags,
	}

	// Create version record
	var changelog *string
	if req.Changelog != "" {
		changelog = &req.Changelog
	}
	fileKey := filepath.Join(assetID.String(), req.Version, safeFilename)
	av := &model.AssetVersion{
		ID:        uuid.New(),
		AssetID:   assetID,
		Version:   req.Version,
		FileKey:   fileKey,
		FileSize:  written,
		Changelog: changelog,
	}

	// Create asset and version in a single transaction
	if err := h.assetRepo.CreateWithVersion(c.Request.Context(), asset, av); err != nil {
		os.RemoveAll(filepath.Join(h.storagePath, assetID.String()))
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create asset")
		return
	}

	// Set current version
	if err := h.assetRepo.UpdateCurrentVersion(c.Request.Context(), assetID, req.Version); err != nil {
		log.Printf("warn: failed to set current version for asset %s: %v", assetID, err)
	}

	// Fetch created asset with author name
	created, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch created asset")
		return
	}
	middleware.RespondCreated(c, created)
}

func (h *AssetHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	sort := c.DefaultQuery("sort", "created_at")
	if sort != "downloads" && sort != "created_at" {
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
	if author := c.Query("author"); author != "" {
		if _, err := uuid.Parse(author); err == nil {
			filter.AuthorID = author
		}
	}

	assets, total, err := h.assetRepo.List(c.Request.Context(), filter)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list assets")
		return
	}

	if assets == nil {
		assets = []model.Asset{}
	}

	if h.favoriteRepo != nil {
		if userIDStr := c.GetString(middleware.ContextUserID); userIDStr != "" {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				favIDs, err := h.favoriteRepo.ListAssetIDs(c.Request.Context(), uid)
				if err == nil {
					favSet := make(map[uuid.UUID]struct{}, len(favIDs))
					for _, id := range favIDs {
						favSet[id] = struct{}{}
					}
					for i := range assets {
						if _, ok := favSet[assets[i].ID]; ok {
							assets[i].IsFavorited = true
						}
					}
				}
			}
		}
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
		if errors.Is(err, repository.ErrAssetNotFound) {
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

	if h.favoriteRepo != nil && userID != "" {
		if uid, err := uuid.Parse(userID); err == nil {
			if exists, err := h.favoriteRepo.Exists(c.Request.Context(), uid, id); err == nil {
				asset.IsFavorited = exists
			}
		}
	}

	middleware.RespondOK(c, asset)
}

func (h *AssetHandler) Download(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	// Check asset exists and authorization
	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	if asset.Status != model.AssetStatusApproved && asset.AuthorID.String() != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Access denied")
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
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset version not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	filePath := filepath.Join(h.storagePath, av.FileKey)
	c.FileAttachment(filePath, fmt.Sprintf("%s-%s.zip", id, av.Version))

	uid, err := uuid.Parse(userID)
	if err == nil {
		go h.assetRepo.IncrementDownloadCount(context.Background(), id, uid)
	}
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

func (h *AssetHandler) UploadVersion(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	if asset.AuthorID.String() != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can upload new versions")
		return
	}

	version := c.PostForm("version")
	if version == "" {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Version is required")
		return
	}
	if !versionRegex.MatchString(version) {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Version must be in format x.y.z (e.g. 1.0.0)")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "File is required")
		return
	}
	defer file.Close()

	if header.Size > maxFileSize {
		middleware.RespondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "File size exceeds 50MB limit")
		return
	}

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Only .zip files are allowed")
		return
	}

	buf := make([]byte, 2)
	n, _ := file.Read(buf)
	if n < 2 || buf[0] != 0x50 || buf[1] != 0x4B {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_FILE", "File is not a valid ZIP archive")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process file")
		return
	}

	safeFilename := filepath.Base(header.Filename)

	dir := filepath.Join(h.storagePath, id.String(), version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create storage directory")
		return
	}

	filePath := filepath.Join(dir, safeFilename)
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

	changelog := c.PostForm("changelog")
	var changelogPtr *string
	if changelog != "" {
		changelogPtr = &changelog
	}

	fileKey := filepath.Join(id.String(), version, safeFilename)
	av := &model.AssetVersion{
		ID:        uuid.New(),
		AssetID:   id,
		Version:   version,
		FileKey:   fileKey,
		FileSize:  written,
		Changelog: changelogPtr,
	}

	if err := h.versionRepo.Create(c.Request.Context(), av); err != nil {
		os.RemoveAll(dir)
		if errors.Is(err, repository.ErrDuplicateVersion) {
			middleware.RespondError(c, http.StatusConflict, "CONFLICT", "Version already exists for this asset")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create version")
		return
	}

	middleware.RespondCreated(c, av)
}

func (h *AssetHandler) SetCurrentVersion(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	if asset.AuthorID.String() != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can change the current version")
		return
	}

	var req struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	_, err = h.versionRepo.GetByVersion(c.Request.Context(), id, req.Version)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Version not found for this asset")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	if err := h.assetRepo.UpdateCurrentVersion(c.Request.Context(), id, req.Version); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update current version")
		return
	}

	updated, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch updated asset")
		return
	}

	middleware.RespondOK(c, updated)
}

func (h *AssetHandler) ToggleFavorite(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get asset")
		return
	}

	if asset.Status != model.AssetStatusApproved {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset not found")
		return
	}

	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	favorited, err := h.favoriteRepo.Toggle(c.Request.Context(), uid, id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to toggle favorite")
		return
	}

	middleware.RespondOK(c, gin.H{"favorited": favorited})
}

package handler

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html"
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
	"github.com/vanzheng/kodaclaw-community/internal/security"
)

const maxFileSize = 50 * 1024 * 1024 // 50MB

var versionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

type AssetHandler struct {
	assetRepo    repository.AssetRepository
	versionRepo  repository.AssetVersionRepository
	userRepo     repository.UserRepository
	favoriteRepo repository.FavoriteRepository
	depRepo      repository.AssetDependencyRepository
	installRepo  repository.AssetInstallRepository
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

func NewAssetHandlerFull(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, userRepo repository.UserRepository, favoriteRepo repository.FavoriteRepository, depRepo repository.AssetDependencyRepository, installRepo repository.AssetInstallRepository, storagePath string) *AssetHandler {
	return &AssetHandler{
		assetRepo:    assetRepo,
		versionRepo:  versionRepo,
		userRepo:     userRepo,
		favoriteRepo: favoriteRepo,
		depRepo:      depRepo,
		installRepo:  installRepo,
		storagePath:  storagePath,
	}
}

const maxReadmeSize = 100 * 1024 // 100KB

// extractZipContent reads a saved zip file and returns content of README.md, SKILL.md, or SOUL.md if present.
// For skill assets: extracts README.md and SKILL.md.
// For soul assets: extracts SOUL.md (stored in skillContent field for reuse).
func extractZipContent(filePath string) (readme, skillContent *string) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, nil
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name != "README.md" && name != "SKILL.md" && name != "SOUL.md" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxReadmeSize))
		rc.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		s := string(data)
		if name == "README.md" {
			readme = &s
		} else if name == "SKILL.md" || name == "SOUL.md" {
			skillContent = &s
		}
	}
	return readme, skillContent
}

// Create godoc
// @Summary 上传新资产
// @Description 上传新的 soul/skill 资产（zip 文件）
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param name formData string true "资产名称"
// @Param type formData string true "资产类型 (soul/skill)"
// @Param description formData string true "资产描述"
// @Param version formData string true "初始版本 (x.y.z)"
// @Param tags formData string false "标签，逗号分隔"
// @Param changelog formData string false "变更日志"
// @Param file formData file true "zip 文件"
// @Success 201 {object} model.Asset
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 413 {object} middleware.ErrorResponse
// @Router /assets [post]
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
	dst.Close()

	if err := security.ScanZip(filePath); err != nil {
		os.Remove(filePath)
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_FILE", err.Error())
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

	// Extract zip content (README.md and SKILL.md)
	readme, skillContent := extractZipContent(filePath)

	// Create asset record
	asset := &model.Asset{
		ID:           assetID,
		Name:         html.EscapeString(req.Name),
		Type:         req.Type,
		Description:  html.EscapeString(req.Description),
		AuthorID:     uid,
		Status:       model.AssetStatusPending,
		Tags:         tags,
		Readme:       readme,
		SkillContent: skillContent,
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

// List godoc
// @Summary 获取资产列表
// @Description 分页获取资产列表，支持过滤和排序
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param type query string false "资产类型 (soul/skill)"
// @Param tag query string false "标签过滤"
// @Param q query string false "搜索关键词"
// @Param author query string false "作者 UUID"
// @Param sort query string false "排序字段 (created_at/downloads/rating)" default(created_at)
// @Success 200 {object} model.AssetListResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /assets [get]
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

// GetByID godoc
// @Summary 获取资产详情
// @Description 通过 ID 获取单个资产的详细信息
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {object} model.Asset
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id} [get]
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

// Download godoc
// @Summary 下载资产文件
// @Description 下载资产的 zip 文件
// @Tags assets
// @Produce application/zip
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param version query string false "版本号，不填则下载当前版本"
// @Success 200 {file} binary
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id}/download [get]
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

// ListVersions godoc
// @Summary 获取资产版本列表
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {array} model.AssetVersion
// @Failure 400 {object} middleware.ErrorResponse
// @Router /assets/{id}/versions [get]
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

// UploadVersion godoc
// @Summary 上传新版本
// @Description 为已有资产上传新版本文件
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param version formData string true "版本号 (x.y.z)"
// @Param changelog formData string false "变更日志"
// @Param file formData file true "zip 文件"
// @Success 201 {object} model.AssetVersion
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 409 {object} middleware.ErrorResponse
// @Router /assets/{id}/versions [post]
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
	dst.Close()

	if err := security.ScanZip(filePath); err != nil {
		os.Remove(filePath)
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_FILE", err.Error())
		return
	}

	changelog := html.EscapeString(c.PostForm("changelog"))
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

	// Extract and update zip content for new version
	readme, skillContent := extractZipContent(filePath)
	if readme != nil || skillContent != nil {
		if err := h.assetRepo.UpdateReadme(c.Request.Context(), id, readme, skillContent); err != nil {
			log.Printf("warn: failed to update readme for asset %s: %v", id, err)
		}
	}

	middleware.RespondCreated(c, av)
}

// SetCurrentVersion godoc
// @Summary 设置当前版本
// @Description 将指定版本设为当前版本
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body object true "版本信息 {version}"
// @Success 200 {object} model.Asset
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id}/versions/current [patch]
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

// Update godoc
// @Summary 更新资产信息
// @Description 更新资产名称、描述和标签
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body object true "更新信息 {name, description, tags}"
// @Success 200 {object} model.Asset
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id} [patch]
func (h *AssetHandler) Update(c *gin.Context) {
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
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can update this asset")
		return
	}

	var req struct {
		Name        string   `json:"name" binding:"required,max=200"`
		Description string   `json:"description" binding:"required"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if len(req.Tags) > 10 {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Maximum 10 tags allowed")
		return
	}
	for _, tag := range req.Tags {
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

	newStatus := asset.Status
	if asset.Status == model.AssetStatusApproved {
		newStatus = model.AssetStatusPending
	}

	if err := h.assetRepo.Update(c.Request.Context(), id, html.EscapeString(req.Name), html.EscapeString(req.Description), req.Tags, newStatus); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update asset")
		return
	}

	updated, err := h.assetRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch updated asset")
		return
	}

	middleware.RespondOK(c, updated)
}

// Delete godoc
// @Summary 删除资产
// @Description 删除资产及其所有版本文件
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id} [delete]
func (h *AssetHandler) Delete(c *gin.Context) {
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
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can delete this asset")
		return
	}

	if err := h.assetRepo.Delete(c.Request.Context(), id); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete asset")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "Asset deleted successfully"})
}

// PopularTags godoc
// @Summary 获取热门标签
// @Description 获取使用频率最高的标签列表
// @Tags tags
// @Produce json
// @Security BearerAuth
// @Success 200 {array} repository.TagCount
// @Failure 500 {object} middleware.ErrorResponse
// @Router /tags/popular [get]
func (h *AssetHandler) PopularTags(c *gin.Context) {
	tags, err := h.assetRepo.PopularTags(c.Request.Context(), 20)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get popular tags")
		return
	}
	if tags == nil {
		tags = []repository.TagCount{}
	}
	middleware.RespondOK(c, tags)
}

// ToggleFavorite godoc
// @Summary 收藏/取消收藏资产
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id}/favorite [post]
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

// ===== Dependencies =====

// AddDependency godoc
// @Summary 添加资产依赖
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body object true "依赖信息 {asset_id}"
// @Success 201 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 409 {object} middleware.ErrorResponse
// @Router /assets/{id}/dependencies [post]
func (h *AssetHandler) AddDependency(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
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
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can manage dependencies")
		return
	}

	var req struct {
		AssetID string `json:"asset_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	depID, err := uuid.Parse(req.AssetID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid dependency asset ID")
		return
	}

	if depID == assetID {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Asset cannot depend on itself")
		return
	}

	if err := h.depRepo.Add(c.Request.Context(), assetID, depID); err != nil {
		if errors.Is(err, repository.ErrDependencyExists) {
			middleware.RespondError(c, http.StatusConflict, "CONFLICT", "Dependency already exists")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add dependency")
		return
	}

	middleware.RespondCreated(c, gin.H{"message": "Dependency added"})
}

// ListDependencies godoc
// @Summary 获取资产依赖列表
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Success 200 {array} model.AssetDependency
// @Failure 400 {object} middleware.ErrorResponse
// @Router /assets/{id}/dependencies [get]
func (h *AssetHandler) ListDependencies(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	deps, err := h.depRepo.List(c.Request.Context(), assetID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list dependencies")
		return
	}

	if deps == nil {
		deps = []model.AssetDependency{}
	}
	middleware.RespondOK(c, deps)
}

// DeleteDependency godoc
// @Summary 移除资产依赖
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param dep_id path string true "依赖资产 UUID"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id}/dependencies/{dep_id} [delete]
func (h *AssetHandler) DeleteDependency(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
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
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Only the author can manage dependencies")
		return
	}

	depID, err := uuid.Parse(c.Param("dep_id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid dependency ID")
		return
	}

	if err := h.depRepo.Delete(c.Request.Context(), assetID, depID); err != nil {
		if errors.Is(err, repository.ErrDependencyNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Dependency not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete dependency")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "Dependency removed"})
}

// ===== Install =====

// InstallAsset godoc
// @Summary 安装资产
// @Description 记录用户安装资产，更新安装计数
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "资产 UUID"
// @Param body body object false "安装信息 {instance_id}"
// @Success 200 {object} object
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Router /assets/{id}/install [post]
func (h *AssetHandler) InstallAsset(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid asset ID")
		return
	}

	asset, err := h.assetRepo.GetByID(c.Request.Context(), assetID)
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

	var req struct {
		InstanceID string `json:"instance_id"`
	}
	c.ShouldBindJSON(&req)

	var instanceID *string
	if req.InstanceID != "" {
		instanceID = &req.InstanceID
	}

	isNew, err := h.installRepo.Install(c.Request.Context(), assetID, uid, instanceID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to record install")
		return
	}

	middleware.RespondOK(c, gin.H{"installed": true, "new": isNew})
}

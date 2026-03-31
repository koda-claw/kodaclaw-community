package model

import (
	"time"

	"github.com/google/uuid"
)

type AssetType string
type AssetStatus string

const (
	AssetTypeSoul  AssetType = "soul"
	AssetTypeSkill AssetType = "skill"

	AssetStatusPending  AssetStatus = "pending"
	AssetStatusApproved AssetStatus = "approved"
	AssetStatusRejected AssetStatus = "rejected"
)

type Asset struct {
	ID               uuid.UUID   `json:"id"`
	Name             string      `json:"name"`
	Type             AssetType   `json:"type"`
	Description      string      `json:"description"`
	AuthorID         uuid.UUID   `json:"author_id"`
	AuthorName       string      `json:"author_name,omitempty"`
	Status           AssetStatus `json:"status"`
	Tags             []string    `json:"tags"`
	CurrentVersion   *string     `json:"current_version,omitempty"`
	RejectionReason  *string     `json:"rejection_reason,omitempty"`
	DownloadCount    int         `json:"download_count"`
	AvgRating        float64     `json:"avg_rating"`
	InstallCount     int         `json:"install_count"`
	Readme           *string     `json:"readme,omitempty"`
	SkillContent     *string     `json:"skill_content,omitempty"`
	IsFavorited      bool        `json:"is_favorited,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type AssetDependency struct {
	AssetID          uuid.UUID `json:"asset_id"`
	DependsOnAssetID uuid.UUID `json:"depends_on_asset_id"`
	Name             string    `json:"name,omitempty"`
	Type             AssetType `json:"type,omitempty"`
}

type AssetVersion struct {
	ID        uuid.UUID `json:"id"`
	AssetID   uuid.UUID `json:"asset_id"`
	Version   string    `json:"version"`
	FileKey   string    `json:"file_key"`
	FileSize  int64     `json:"file_size"`
	Changelog *string   `json:"changelog,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type UploadAssetRequest struct {
	Name        string   `form:"name" binding:"required,max=200"`
	Type        AssetType `form:"type" binding:"required,oneof=soul skill"`
	Description string   `form:"description" binding:"required"`
	Tags        string   `form:"tags"`        // comma-separated
	Version     string   `form:"version" binding:"required"`
	Changelog   string   `form:"changelog"`
}

type AssetListResponse struct {
	Items    []Asset `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

type RejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

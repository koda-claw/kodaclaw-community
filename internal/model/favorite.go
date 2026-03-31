package model

import (
	"time"

	"github.com/google/uuid"
)

type Favorite struct {
	UserID    uuid.UUID `json:"user_id"`
	AssetID   uuid.UUID `json:"asset_id"`
	AssetName string    `json:"asset_name,omitempty"`
	AssetType string    `json:"asset_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type FavoriteListResponse struct {
	Items    []Favorite `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

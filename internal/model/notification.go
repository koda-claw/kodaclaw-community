package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Message        *string    `json:"message,omitempty"`
	RelatedAssetID *uuid.UUID `json:"related_asset_id,omitempty"`
	IsRead         bool       `json:"is_read"`
	CreatedAt      time.Time  `json:"created_at"`
}

type NotificationListResponse struct {
	Items    []Notification `json:"items"`
	Total    int            `json:"total"`
	Unread   int            `json:"unread"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

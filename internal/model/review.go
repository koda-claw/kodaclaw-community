package model

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID            uuid.UUID `json:"id"`
	AssetID       uuid.UUID `json:"asset_id"`
	UserID        uuid.UUID `json:"user_id"`
	Username      string    `json:"username"`
	Content       string    `json:"content"`
	Compatibility *int      `json:"compatibility,omitempty"`
	Usefulness    *int      `json:"usefulness,omitempty"`
	Security      *int      `json:"security,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateReviewRequest struct {
	Content       string `json:"content" binding:"required,max=2000"`
	Compatibility *int   `json:"compatibility" binding:"omitempty,gte=1,lte=5"`
	Usefulness    *int   `json:"usefulness" binding:"omitempty,gte=1,lte=5"`
	Security      *int   `json:"security" binding:"omitempty,gte=1,lte=5"`
}

type ReviewListResponse struct {
	Items    []Review `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

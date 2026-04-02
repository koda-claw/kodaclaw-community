package model

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         uuid.UUID `json:"id"`
	OperatorID uuid.UUID `json:"operator_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   uuid.UUID `json:"target_id"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserActivity struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	ActivityType string     `json:"activity_type"`
	AssetID      *uuid.UUID `json:"asset_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

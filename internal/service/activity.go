package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type ActivityService struct {
	repo *repository.UserActivityRepo
}

func NewActivityService(repo *repository.UserActivityRepo) *ActivityService {
	return &ActivityService{repo: repo}
}

// Record writes a user activity entry. Errors are only slog.Warn'd, never returned.
func (s *ActivityService) Record(ctx context.Context, userID uuid.UUID, activityType string, assetID *uuid.UUID) {
	a := &model.UserActivity{
		ID:           uuid.New(),
		UserID:       userID,
		ActivityType: activityType,
		AssetID:      assetID,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, a); err != nil {
		slog.Warn("activity record write failed", "error", err, "activity_type", activityType)
	}
}

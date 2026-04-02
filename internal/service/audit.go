package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type AuditService struct {
	repo *repository.AuditLogRepo
}

func NewAuditService(repo *repository.AuditLogRepo) *AuditService {
	return &AuditService{repo: repo}
}

// Log writes an audit log entry. Errors are only slog.Warn'd, never returned.
func (s *AuditService) Log(ctx context.Context, operatorID uuid.UUID, action, targetType string, targetID uuid.UUID, detail string) {
	entry := &model.AuditLog{
		ID:         uuid.New(),
		OperatorID: operatorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		slog.Warn("audit log write failed", "error", err, "action", action)
	}
}

// List returns paginated audit log entries.
func (s *AuditService) List(ctx context.Context, page, pageSize int) ([]model.AuditLog, int, error) {
	return s.repo.List(ctx, page, pageSize)
}

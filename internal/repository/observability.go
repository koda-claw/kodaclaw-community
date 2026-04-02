package repository

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

type AuditLogRepo struct{ pool *pgxpool.Pool }
type UserActivityRepo struct{ pool *pgxpool.Pool }

func NewAuditLogRepo(pool *pgxpool.Pool) *AuditLogRepo {
	return &AuditLogRepo{pool: pool}
}

func NewUserActivityRepo(pool *pgxpool.Pool) *UserActivityRepo {
	return &UserActivityRepo{pool: pool}
}

func (r *AuditLogRepo) Create(ctx context.Context, l *model.AuditLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs (id, operator_id, action, target_type, target_id, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		l.ID, l.OperatorID, l.Action, l.TargetType, l.TargetID, l.Detail, l.CreatedAt,
	)
	return err
}

func (r *AuditLogRepo) List(ctx context.Context, page, pageSize int) ([]model.AuditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, operator_id, action, target_type, target_id, detail, created_at
		 FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var e model.AuditLog
		if err := rows.Scan(&e.ID, &e.OperatorID, &e.Action, &e.TargetType, &e.TargetID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, e)
	}
	if logs == nil {
		logs = []model.AuditLog{}
	}
	return logs, total, nil
}

func (r *UserActivityRepo) Create(ctx context.Context, a *model.UserActivity) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_activities (id, user_id, activity_type, asset_id, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		a.ID, a.UserID, a.ActivityType, a.AssetID, a.CreatedAt,
	)
	return err
}

// RunObservabilityMigrations creates audit_logs and user_activities tables if they don't exist.
func RunObservabilityMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			operator_id UUID NOT NULL,
			action VARCHAR(100) NOT NULL,
			target_type VARCHAR(50) NOT NULL,
			target_id UUID NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_operator ON audit_logs(operator_id)`,
		`CREATE TABLE IF NOT EXISTS user_activities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			activity_type VARCHAR(50) NOT NULL,
			asset_id UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_activities_user ON user_activities(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_activities_asset ON user_activities(asset_id)`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			slog.Warn("observability migration failed", "error", err)
			return err
		}
	}
	return nil
}

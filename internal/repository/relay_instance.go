package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var ErrRelayInstanceNotFound = errors.New("relay instance not found")

func RunRelayMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS relay_instances (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id),
			instance_name VARCHAR(100) NOT NULL,
			account_id VARCHAR(200) NOT NULL UNIQUE,
			shared_secret VARCHAR(200) NOT NULL,
			is_active BOOLEAN DEFAULT true,
			last_connected_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_relay_instances_user ON relay_instances(user_id)`,
		`ALTER TABLE relay_instances ADD COLUMN IF NOT EXISTS webhook_secret VARCHAR(200) NOT NULL DEFAULT ''`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

type RelayInstanceRepository interface {
	CreateRelayInstance(ctx context.Context, instance *model.RelayInstance) error
	GetRelayInstanceByID(ctx context.Context, id string) (*model.RelayInstance, error)
	ListRelayInstancesByUserID(ctx context.Context, userID string) ([]model.RelayInstance, error)
	GetRelayInstanceByAccountID(ctx context.Context, accountID string) (*model.RelayInstance, error)
	UpdateLastConnectedAt(ctx context.Context, id string, t time.Time) error
	UpdateSharedSecret(ctx context.Context, id string, hashedSecret string) error
	UpdateWebhookSecret(ctx context.Context, id string, secret string) error
	DeleteRelayInstance(ctx context.Context, id string) error
}

type relayInstanceRepo struct {
	pool *pgxpool.Pool
}

func NewRelayInstanceRepository(pool *pgxpool.Pool) RelayInstanceRepository {
	return &relayInstanceRepo{pool: pool}
}

func (r *relayInstanceRepo) CreateRelayInstance(ctx context.Context, instance *model.RelayInstance) error {
	// Auto-generate webhook_secret
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	instance.WebhookSecret = "whsec-" + hex.EncodeToString(b)

	_, err := r.pool.Exec(ctx,
		`INSERT INTO relay_instances (id, user_id, instance_name, account_id, shared_secret, webhook_secret, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		instance.UserID, instance.InstanceName, instance.AccountID, instance.SharedSecret, instance.WebhookSecret, instance.IsActive,
	)
	if err != nil {
		return err
	}
	// Re-fetch to get the generated ID and timestamps
	row := r.pool.QueryRow(ctx,
		`SELECT id, created_at, updated_at FROM relay_instances WHERE account_id = $1`,
		instance.AccountID,
	)
	return row.Scan(&instance.ID, &instance.CreatedAt, &instance.UpdatedAt)
}

func (r *relayInstanceRepo) GetRelayInstanceByID(ctx context.Context, id string) (*model.RelayInstance, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, instance_name, account_id, shared_secret, webhook_secret, is_active, last_connected_at, created_at, updated_at
		 FROM relay_instances WHERE id = $1`,
		id,
	)
	return scanRelayInstance(row)
}

func (r *relayInstanceRepo) ListRelayInstancesByUserID(ctx context.Context, userID string) ([]model.RelayInstance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, instance_name, account_id, shared_secret, webhook_secret, is_active, last_connected_at, created_at, updated_at
		 FROM relay_instances WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.RelayInstance
	for rows.Next() {
		var inst model.RelayInstance
		if err := rows.Scan(
			&inst.ID, &inst.UserID, &inst.InstanceName, &inst.AccountID, &inst.SharedSecret, &inst.WebhookSecret,
			&inst.IsActive, &inst.LastConnectedAt, &inst.CreatedAt, &inst.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, inst)
	}
	return results, rows.Err()
}

func (r *relayInstanceRepo) GetRelayInstanceByAccountID(ctx context.Context, accountID string) (*model.RelayInstance, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, instance_name, account_id, shared_secret, webhook_secret, is_active, last_connected_at, created_at, updated_at
		 FROM relay_instances WHERE account_id = $1`,
		accountID,
	)
	return scanRelayInstance(row)
}

func (r *relayInstanceRepo) UpdateLastConnectedAt(ctx context.Context, id string, t time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_instances SET last_connected_at = $1, updated_at = NOW() WHERE id = $2`,
		t, id,
	)
	return err
}

func (r *relayInstanceRepo) UpdateSharedSecret(ctx context.Context, id string, hashedSecret string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_instances SET shared_secret = $1, updated_at = NOW() WHERE id = $2`,
		hashedSecret, id,
	)
	return err
}

func (r *relayInstanceRepo) UpdateWebhookSecret(ctx context.Context, id string, secret string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_instances SET webhook_secret = $1, updated_at = NOW() WHERE id = $2`,
		secret, id,
	)
	return err
}

func (r *relayInstanceRepo) DeleteRelayInstance(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM relay_instances WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRelayInstanceNotFound
	}
	return nil
}

func scanRelayInstance(row pgx.Row) (*model.RelayInstance, error) {
	var inst model.RelayInstance
	err := row.Scan(
		&inst.ID, &inst.UserID, &inst.InstanceName, &inst.AccountID, &inst.SharedSecret, &inst.WebhookSecret,
		&inst.IsActive, &inst.LastConnectedAt, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRelayInstanceNotFound
		}
		return nil, err
	}
	return &inst, nil
}

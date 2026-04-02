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

var ErrWebhookKeyNotFound = errors.New("webhook key not found")

// WebhookKeyWithInstance 包含 key 信息及其所属实例信息
type WebhookKeyWithInstance struct {
	model.RelayWebhookKey
	UserID       string
	AccountID    string
	InstanceName string
}

type WebhookKeyRepository interface {
	ListWebhookKeys(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error)
	GetWebhookKey(ctx context.Context, keyID string) (*model.RelayWebhookKey, error)
	CreateWebhookKey(ctx context.Context, key *model.RelayWebhookKey) error
	DeleteWebhookKey(ctx context.Context, keyID string) error
	UpdateWebhookKeyActive(ctx context.Context, keyID string, isActive bool) error
	GetActiveKeysForVerification(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error)
	// GetExpiringKeys 查找 expires_at <= before 的 active key，并过滤掉已通知的 key
	GetExpiringKeys(ctx context.Context, before time.Time, notifiedKeyIDs map[string]bool) ([]WebhookKeyWithInstance, error)
}

type webhookKeyRepo struct {
	pool *pgxpool.Pool
}

func NewWebhookKeyRepository(pool *pgxpool.Pool) WebhookKeyRepository {
	return &webhookKeyRepo{pool: pool}
}

func (r *webhookKeyRepo) ListWebhookKeys(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, instance_id, key_name, key_prefix, is_active, expires_at, created_at
		 FROM relay_webhook_keys WHERE instance_id = $1 ORDER BY created_at DESC`,
		instanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.RelayWebhookKey
	for rows.Next() {
		var k model.RelayWebhookKey
		if err := rows.Scan(&k.ID, &k.InstanceID, &k.KeyName, &k.KeyPrefix, &k.IsActive, &k.ExpiresAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, k)
	}
	return results, rows.Err()
}

func (r *webhookKeyRepo) GetWebhookKey(ctx context.Context, keyID string) (*model.RelayWebhookKey, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, instance_id, key_name, key_value, key_prefix, is_active, expires_at, created_at
		 FROM relay_webhook_keys WHERE id = $1`,
		keyID,
	)
	var k model.RelayWebhookKey
	err := row.Scan(&k.ID, &k.InstanceID, &k.KeyName, &k.KeyValue, &k.KeyPrefix, &k.IsActive, &k.ExpiresAt, &k.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWebhookKeyNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *webhookKeyRepo) CreateWebhookKey(ctx context.Context, key *model.RelayWebhookKey) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	key.KeyValue = "whsec-" + hex.EncodeToString(b)
	// KeyPrefix: "whsec-" (6) + 8 hex chars = 14 chars total, then "..."
	key.KeyPrefix = key.KeyValue[:14] + "..."

	row := r.pool.QueryRow(ctx,
		`INSERT INTO relay_webhook_keys (instance_id, key_name, key_value, key_prefix, is_active, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		key.InstanceID, key.KeyName, key.KeyValue, key.KeyPrefix, key.IsActive, key.ExpiresAt,
	)
	return row.Scan(&key.ID, &key.CreatedAt)
}

func (r *webhookKeyRepo) DeleteWebhookKey(ctx context.Context, keyID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM relay_webhook_keys WHERE id = $1`, keyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookKeyNotFound
	}
	return nil
}

func (r *webhookKeyRepo) UpdateWebhookKeyActive(ctx context.Context, keyID string, isActive bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE relay_webhook_keys SET is_active = $1 WHERE id = $2`,
		isActive, keyID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookKeyNotFound
	}
	return nil
}

func (r *webhookKeyRepo) GetActiveKeysForVerification(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, instance_id, key_name, key_value, key_prefix, is_active, expires_at, created_at
		 FROM relay_webhook_keys
		 WHERE instance_id = $1 AND is_active = true AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY created_at DESC`,
		instanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.RelayWebhookKey
	for rows.Next() {
		var k model.RelayWebhookKey
		if err := rows.Scan(&k.ID, &k.InstanceID, &k.KeyName, &k.KeyValue, &k.KeyPrefix, &k.IsActive, &k.ExpiresAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, k)
	}
	return results, rows.Err()
}

func (r *webhookKeyRepo) GetExpiringKeys(ctx context.Context, before time.Time, notifiedKeyIDs map[string]bool) ([]WebhookKeyWithInstance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT wk.id, wk.instance_id, wk.key_name, wk.key_prefix, wk.is_active, wk.expires_at, wk.created_at,
		        ri.user_id, ri.account_id, ri.instance_name
		 FROM relay_webhook_keys wk
		 JOIN relay_instances ri ON wk.instance_id = ri.id
		 WHERE wk.is_active = true
		   AND wk.expires_at IS NOT NULL
		   AND wk.expires_at <= $1
		 ORDER BY wk.expires_at ASC`,
		before,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WebhookKeyWithInstance
	for rows.Next() {
		var k WebhookKeyWithInstance
		if err := rows.Scan(
			&k.ID, &k.InstanceID, &k.KeyName, &k.KeyPrefix, &k.IsActive, &k.ExpiresAt, &k.CreatedAt,
			&k.UserID, &k.AccountID, &k.InstanceName,
		); err != nil {
			return nil, err
		}
		if !notifiedKeyIDs[k.ID] {
			results = append(results, k)
		}
	}
	return results, rows.Err()
}

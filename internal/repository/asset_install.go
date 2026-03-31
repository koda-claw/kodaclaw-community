package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AssetInstallRepository interface {
	Install(ctx context.Context, assetID, userID uuid.UUID, instanceID *string) (bool, error)
}

type assetInstallRepo struct {
	pool *pgxpool.Pool
}

func NewAssetInstallRepository(pool *pgxpool.Pool) AssetInstallRepository {
	return &assetInstallRepo{pool: pool}
}

// Install records an install. Returns true if a new install was recorded, false if already installed.
func (r *assetInstallRepo) Install(ctx context.Context, assetID, userID uuid.UUID, instanceID *string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`INSERT INTO asset_installs (asset_id, user_id, instance_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		assetID, userID, instanceID)
	if err != nil {
		return false, err
	}

	if tag.RowsAffected() == 0 {
		return false, nil
	}

	_, err = tx.Exec(ctx,
		`UPDATE assets SET install_count = install_count + 1 WHERE id = $1`,
		assetID)
	if err != nil {
		return false, err
	}

	return true, tx.Commit(ctx)
}

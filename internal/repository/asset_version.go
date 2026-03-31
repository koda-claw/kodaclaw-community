package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

type AssetVersionRepository interface {
	Create(ctx context.Context, av *model.AssetVersion) error
	GetByVersion(ctx context.Context, assetID uuid.UUID, version string) (*model.AssetVersion, error)
	GetCurrent(ctx context.Context, assetID uuid.UUID) (*model.AssetVersion, error)
	ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]model.AssetVersion, error)
}

type assetVersionRepo struct {
	pool *pgxpool.Pool
}

func NewAssetVersionRepository(pool *pgxpool.Pool) AssetVersionRepository {
	return &assetVersionRepo{pool: pool}
}

func (r *assetVersionRepo) Create(ctx context.Context, av *model.AssetVersion) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO asset_versions (id, asset_id, version, file_key, file_size, changelog)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		av.ID, av.AssetID, av.Version, av.FileKey, av.FileSize, av.Changelog)
	return err
}

func (r *assetVersionRepo) GetByVersion(ctx context.Context, assetID uuid.UUID, version string) (*model.AssetVersion, error) {
	var av model.AssetVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, asset_id, version, file_key, file_size, changelog, created_at
		 FROM asset_versions WHERE asset_id = $1 AND version = $2`, assetID, version).
		Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &av, err
}

func (r *assetVersionRepo) GetCurrent(ctx context.Context, assetID uuid.UUID) (*model.AssetVersion, error) {
	var av model.AssetVersion
	err := r.pool.QueryRow(ctx,
		`SELECT v.id, v.asset_id, v.version, v.file_key, v.file_size, v.changelog, v.created_at
		 FROM asset_versions v
		 JOIN assets a ON a.id = v.asset_id
		 WHERE a.id = $1 AND a.current_version = v.version`, assetID).
		Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &av, err
}

func (r *assetVersionRepo) ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]model.AssetVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset_id, version, file_key, file_size, changelog, created_at
		 FROM asset_versions WHERE asset_id = $1 ORDER BY created_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []model.AssetVersion
	for rows.Next() {
		var av model.AssetVersion
		if err := rows.Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, av)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

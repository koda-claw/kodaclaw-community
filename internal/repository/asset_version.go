package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var ErrDuplicateVersion = errors.New("version already exists for this asset")

type AssetVersionRepository interface {
	Create(ctx context.Context, av *model.AssetVersion) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AssetVersion, error)
	GetByVersion(ctx context.Context, assetID uuid.UUID, version string) (*model.AssetVersion, error)
	GetCurrent(ctx context.Context, assetID uuid.UUID) (*model.AssetVersion, error)
	ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]model.AssetVersion, error)
	ListAllFileKeys(ctx context.Context) ([]string, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason *string) error
	ListPending(ctx context.Context, page, pageSize int) ([]model.AssetVersion, int, error)
}

type assetVersionRepo struct {
	pool *pgxpool.Pool
}

func NewAssetVersionRepository(pool *pgxpool.Pool) AssetVersionRepository {
	return &assetVersionRepo{pool: pool}
}

func (r *assetVersionRepo) Create(ctx context.Context, av *model.AssetVersion) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO asset_versions (id, asset_id, version, file_key, file_size, changelog, status, skill_content, readme)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		av.ID, av.AssetID, av.Version, av.FileKey, av.FileSize, av.Changelog, av.Status, av.SkillContent, av.Readme)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateVersion
		}
	}
	return err
}

func (r *assetVersionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.AssetVersion, error) {
	var av model.AssetVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, asset_id, version, file_key, file_size, changelog, status, rejection_reason, skill_content, readme, created_at
		 FROM asset_versions WHERE id = $1`, id).
		Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.Status, &av.RejectionReason, &av.SkillContent, &av.Readme, &av.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &av, err
}

func (r *assetVersionRepo) GetByVersion(ctx context.Context, assetID uuid.UUID, version string) (*model.AssetVersion, error) {
	var av model.AssetVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, asset_id, version, file_key, file_size, changelog, status, rejection_reason, skill_content, readme, created_at
		 FROM asset_versions WHERE asset_id = $1 AND version = $2`, assetID, version).
		Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.Status, &av.RejectionReason, &av.SkillContent, &av.Readme, &av.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &av, err
}

func (r *assetVersionRepo) GetCurrent(ctx context.Context, assetID uuid.UUID) (*model.AssetVersion, error) {
	var av model.AssetVersion
	err := r.pool.QueryRow(ctx,
		`SELECT v.id, v.asset_id, v.version, v.file_key, v.file_size, v.changelog, v.status, v.rejection_reason, v.skill_content, v.readme, v.created_at
		 FROM asset_versions v
		 JOIN assets a ON a.id = v.asset_id
		 WHERE a.id = $1 AND a.current_version = v.version`, assetID).
		Scan(&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.Status, &av.RejectionReason, &av.SkillContent, &av.Readme, &av.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &av, err
}

func (r *assetVersionRepo) ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]model.AssetVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, asset_id, version, file_key, file_size, changelog, status, rejection_reason, skill_content, readme, created_at
		 FROM asset_versions WHERE asset_id = $1 ORDER BY created_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []model.AssetVersion
	for rows.Next() {
		var av model.AssetVersion
		if err := rows.Scan(
			&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.Status, &av.RejectionReason, &av.SkillContent, &av.Readme, &av.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, av)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (r *assetVersionRepo) ListAllFileKeys(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT file_key FROM asset_versions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// UpdateStatus changes the review status of a version.
func (r *assetVersionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE asset_versions SET status = $1, rejection_reason = $2 WHERE id = $3`,
		status, rejectionReason, id)
	return err
}

// ListPending returns pending versions with asset info for admin review.
func (r *assetVersionRepo) ListPending(ctx context.Context, page, pageSize int) ([]model.AssetVersion, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM asset_versions WHERE status = 'pending'`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		`SELECT v.id, v.asset_id, v.version, v.file_key, v.file_size, v.changelog, v.status, v.rejection_reason, v.skill_content, v.readme, v.created_at
		 FROM asset_versions v
		 WHERE v.status = 'pending'
		 ORDER BY v.created_at ASC
		 LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var versions []model.AssetVersion
	for rows.Next() {
		var av model.AssetVersion
		if err := rows.Scan(
			&av.ID, &av.AssetID, &av.Version, &av.FileKey, &av.FileSize, &av.Changelog, &av.Status, &av.RejectionReason, &av.SkillContent, &av.Readme, &av.CreatedAt); err != nil {
			return nil, 0, err
		}
		versions = append(versions, av)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

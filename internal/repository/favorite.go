package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

type FavoriteRepository interface {
	Toggle(ctx context.Context, userID, assetID uuid.UUID) (bool, error)
	Exists(ctx context.Context, userID, assetID uuid.UUID) (bool, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Favorite, int, error)
	ListAssetIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	Delete(ctx context.Context, userID, assetID uuid.UUID) error
}

type favoriteRepo struct {
	pool *pgxpool.Pool
}

func NewFavoriteRepository(pool *pgxpool.Pool) FavoriteRepository {
	return &favoriteRepo{pool: pool}
}

func (r *favoriteRepo) Toggle(ctx context.Context, userID, assetID uuid.UUID) (bool, error) {
	exists, err := r.Exists(ctx, userID, assetID)
	if err != nil {
		return false, err
	}
	if exists {
		if err := r.Delete(ctx, userID, assetID); err != nil {
			return false, err
		}
		return false, nil
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO asset_favorites (user_id, asset_id) VALUES ($1, $2)`,
		userID, assetID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *favoriteRepo) Exists(ctx context.Context, userID, assetID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM asset_favorites WHERE user_id = $1 AND asset_id = $2)`,
		userID, assetID).Scan(&exists)
	return exists, err
}

func (r *favoriteRepo) ListByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Favorite, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM asset_favorites WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		`SELECT f.user_id, f.asset_id, a.name, a.type, f.created_at
		 FROM asset_favorites f JOIN assets a ON f.asset_id = a.id
		 WHERE f.user_id = $1
		 ORDER BY f.created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var favorites []model.Favorite
	for rows.Next() {
		var f model.Favorite
		if err := rows.Scan(&f.UserID, &f.AssetID, &f.AssetName, &f.AssetType, &f.CreatedAt); err != nil {
			return nil, 0, err
		}
		favorites = append(favorites, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return favorites, total, nil
}

func (r *favoriteRepo) ListAssetIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT asset_id FROM asset_favorites WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *favoriteRepo) Delete(ctx context.Context, userID, assetID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM asset_favorites WHERE user_id = $1 AND asset_id = $2`,
		userID, assetID)
	return err
}

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	Update(ctx context.Context, review *model.Review) error
	GetByUserAndAsset(ctx context.Context, assetID, userID uuid.UUID) (*model.Review, error)
	ExistsByUserAndAsset(ctx context.Context, assetID, userID uuid.UUID) (bool, error)
	ListByAssetID(ctx context.Context, assetID uuid.UUID, page, pageSize int) ([]model.Review, int, error)
}

type reviewRepo struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) ReviewRepository {
	return &reviewRepo{pool: pool}
}

func (r *reviewRepo) Create(ctx context.Context, review *model.Review) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO reviews (id, asset_id, user_id, content, compatibility, usefulness, security)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		review.ID, review.AssetID, review.UserID, review.Content,
		review.Compatibility, review.Usefulness, review.Security)
	return err
}

func (r *reviewRepo) ExistsByUserAndAsset(ctx context.Context, assetID, userID uuid.UUID) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reviews WHERE asset_id = $1 AND user_id = $2`,
		assetID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *reviewRepo) ListByAssetID(ctx context.Context, assetID uuid.UUID, page, pageSize int) ([]model.Review, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reviews WHERE asset_id = $1`, assetID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.asset_id, r.user_id, u.username, r.content, r.compatibility, r.usefulness, r.security, r.created_at
		 FROM reviews r JOIN users u ON r.user_id = u.id
		 WHERE r.asset_id = $1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`,
		assetID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reviews []model.Review
	for rows.Next() {
		var rev model.Review
		if err := rows.Scan(&rev.ID, &rev.AssetID, &rev.UserID, &rev.Username,
			&rev.Content, &rev.Compatibility, &rev.Usefulness, &rev.Security, &rev.CreatedAt); err != nil {
			return nil, 0, err
		}
		reviews = append(reviews, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return reviews, total, nil
}

func (r *reviewRepo) Update(ctx context.Context, review *model.Review) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reviews SET content = $1, compatibility = $2, usefulness = $3, security = $4
		 WHERE id = $5`,
		review.Content, review.Compatibility, review.Usefulness, review.Security, review.ID)
	return err
}

func (r *reviewRepo) GetByUserAndAsset(ctx context.Context, assetID, userID uuid.UUID) (*model.Review, error) {
	var rev model.Review
	err := r.pool.QueryRow(ctx,
		`SELECT r.id, r.asset_id, r.user_id, u.username, r.content, r.compatibility, r.usefulness, r.security, r.created_at
		 FROM reviews r JOIN users u ON r.user_id = u.id
		 WHERE r.asset_id = $1 AND r.user_id = $2`,
		assetID, userID).Scan(
		&rev.ID, &rev.AssetID, &rev.UserID, &rev.Username,
		&rev.Content, &rev.Compatibility, &rev.Usefulness, &rev.Security, &rev.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

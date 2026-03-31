package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var ErrAssetNotFound = errors.New("asset not found")

type AssetRepository interface {
	Create(ctx context.Context, asset *model.Asset) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Asset, error)
	List(ctx context.Context, filter AssetFilter) ([]model.Asset, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.AssetStatus, reason *string) error
	UpdateCurrentVersion(ctx context.Context, id uuid.UUID, version string) error
}

type AssetFilter struct {
	Type     string
	Tag      string
	Query    string
	Status   string // empty = public (approved only), set value = admin filter by specific status
	Page     int
	PageSize int
}

type assetRepo struct {
	pool *pgxpool.Pool
}

func NewAssetRepository(pool *pgxpool.Pool) AssetRepository {
	return &assetRepo{pool: pool}
}

func (r *assetRepo) Create(ctx context.Context, asset *model.Asset) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO assets (id, name, type, description, author_id, status, tags, current_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		asset.ID, asset.Name, asset.Type, asset.Description, asset.AuthorID, asset.Status, asset.Tags, asset.CurrentVersion)
	return err
}

func (r *assetRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Asset, error) {
	var a model.Asset
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.name, a.type, a.description, a.author_id, u.username, a.status, a.tags, a.current_version, a.rejection_reason, a.created_at, a.updated_at
		 FROM assets a JOIN users u ON a.author_id = u.id WHERE a.id = $1`, id).
		Scan(&a.ID, &a.Name, &a.Type, &a.Description, &a.AuthorID, &a.AuthorName,
			&a.Status, &a.Tags, &a.CurrentVersion, &a.RejectionReason, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &a, err
}

func (r *assetRepo) List(ctx context.Context, filter AssetFilter) ([]model.Asset, int, error) {
	where := []string{}
	args := []interface{}{}
	argIdx := 1

	// Status filter: empty = public (approved only), set = filter by specific status
	if filter.Status != "" {
		where = append(where, fmt.Sprintf("a.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	} else {
		where = append(where, "a.status = 'approved'")
	}

	if filter.Type != "" {
		where = append(where, fmt.Sprintf("a.type = $%d", argIdx))
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Tag != "" {
		where = append(where, fmt.Sprintf("$%d = ANY(a.tags)", argIdx))
		args = append(args, filter.Tag)
		argIdx++
	}
	if filter.Query != "" {
		where = append(where, fmt.Sprintf("(a.name ILIKE $%d OR a.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Query+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM assets a WHERE %s", whereClause)
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	querySQL := fmt.Sprintf(
		`SELECT a.id, a.name, a.type, a.description, a.author_id, u.username, a.status, a.tags, a.current_version, a.rejection_reason, a.created_at, a.updated_at
		 FROM assets a JOIN users u ON a.author_id = u.id WHERE %s ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	rows, err := r.pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var assets []model.Asset
	for rows.Next() {
		var a model.Asset
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Description, &a.AuthorID, &a.AuthorName,
			&a.Status, &a.Tags, &a.CurrentVersion, &a.RejectionReason, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		assets = append(assets, a)
	}

	return assets, total, nil
}

func (r *assetRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AssetStatus, reason *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET status = $1, rejection_reason = $2, updated_at = NOW() WHERE id = $3`,
		status, reason, id)
	return err
}

func (r *assetRepo) UpdateCurrentVersion(ctx context.Context, id uuid.UUID, version string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET current_version = $1, updated_at = NOW() WHERE id = $2`,
		version, id)
	return err
}

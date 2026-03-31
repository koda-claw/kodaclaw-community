package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var ErrDependencyExists = errors.New("dependency already exists")
var ErrDependencyNotFound = errors.New("dependency not found")

type AssetDependencyRepository interface {
	Add(ctx context.Context, assetID, dependsOnID uuid.UUID) error
	List(ctx context.Context, assetID uuid.UUID) ([]model.AssetDependency, error)
	Delete(ctx context.Context, assetID, dependsOnID uuid.UUID) error
}

type assetDependencyRepo struct {
	pool *pgxpool.Pool
}

func NewAssetDependencyRepository(pool *pgxpool.Pool) AssetDependencyRepository {
	return &assetDependencyRepo{pool: pool}
}

func (r *assetDependencyRepo) Add(ctx context.Context, assetID, dependsOnID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO asset_dependencies (asset_id, depends_on_asset_id) VALUES ($1, $2)`,
		assetID, dependsOnID)
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDependencyExists
		}
		return err
	}
	return nil
}

func (r *assetDependencyRepo) List(ctx context.Context, assetID uuid.UUID) ([]model.AssetDependency, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT d.depends_on_asset_id, a.name, a.type
		 FROM asset_dependencies d
		 JOIN assets a ON d.depends_on_asset_id = a.id
		 WHERE d.asset_id = $1`,
		assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []model.AssetDependency
	for rows.Next() {
		var dep model.AssetDependency
		dep.AssetID = assetID
		if err := rows.Scan(&dep.DependsOnAssetID, &dep.Name, &dep.Type); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deps, nil
}

func (r *assetDependencyRepo) Delete(ctx context.Context, assetID, dependsOnID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM asset_dependencies WHERE asset_id = $1 AND depends_on_asset_id = $2`,
		assetID, dependsOnID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDependencyNotFound
	}
	return nil
}

// isDuplicateKeyError checks if the error is a unique constraint violation.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// pgx wraps pgconn.PgError; check error code 23505
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	// fallback: check pgx.ErrNoRows just in case
	return errors.Is(err, pgx.ErrNoRows)
}

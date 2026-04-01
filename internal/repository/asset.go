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
	CreateWithVersion(ctx context.Context, asset *model.Asset, version *model.AssetVersion) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Asset, error)
	List(ctx context.Context, filter AssetFilter) ([]model.Asset, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.AssetStatus, reason *string) error
	UpdateCurrentVersion(ctx context.Context, id uuid.UUID, version string) error
	IncrementDownloadCount(ctx context.Context, assetID, userID uuid.UUID) error
	UpdateAvgRating(ctx context.Context, assetID uuid.UUID) error
	PopularTags(ctx context.Context, limit int) ([]TagCount, error)
	Update(ctx context.Context, id uuid.UUID, name, description string, tags []string, status model.AssetStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	TotalDownloads(ctx context.Context) (int64, error)
	UpdateReadme(ctx context.Context, id uuid.UUID, readme, skillContent *string) error
	GetByName(ctx context.Context, name string) (*model.Asset, error)
	CountByDay(ctx context.Context, days int) ([]DayCount, error)
	CountDownloadsByDay(ctx context.Context, days int) ([]DayCount, error)
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type DayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type AssetFilter struct {
	Type     string
	Tag      string
	Query    string
	Status   string // empty = public (approved only), set value = admin filter by specific status
	ShowAll  bool   // if true, don't filter by status (for own assets view)
	AuthorID string // filter by author
	Sort     string // "downloads", "created_at", "rating" (default: "created_at")
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

// CreateWithVersion creates an asset and its first version in a single transaction.
// If version creation fails, the asset creation is rolled back.
func (r *assetRepo) CreateWithVersion(ctx context.Context, asset *model.Asset, version *model.AssetVersion) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO assets (id, name, type, description, author_id, status, tags, current_version, asset_readme, asset_skill_content)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		asset.ID, asset.Name, asset.Type, asset.Description, asset.AuthorID, asset.Status, asset.Tags, asset.CurrentVersion, asset.Readme, asset.SkillContent)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO asset_versions (id, asset_id, version, file_key, file_size, changelog, status, skill_content, readme)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		version.ID, version.AssetID, version.Version, version.FileKey, version.FileSize, version.Changelog, version.Status, version.SkillContent, version.Readme)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *assetRepo) UpdateReadme(ctx context.Context, id uuid.UUID, readme, skillContent *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET asset_readme = $1, asset_skill_content = $2, updated_at = NOW() WHERE id = $3`,
		readme, skillContent, id)
	return err
}

func (r *assetRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Asset, error) {
	var a model.Asset
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.name, a.type, a.description, a.author_id, u.username, a.status, a.tags, a.current_version, a.rejection_reason, a.download_count, a.avg_rating, a.install_count, a.asset_readme, a.asset_skill_content, a.created_at, a.updated_at
		 FROM assets a JOIN users u ON a.author_id = u.id WHERE a.id = $1`, id).
		Scan(&a.ID, &a.Name, &a.Type, &a.Description, &a.AuthorID, &a.AuthorName,
			&a.Status, &a.Tags, &a.CurrentVersion, &a.RejectionReason, &a.DownloadCount, &a.AvgRating, &a.InstallCount, &a.Readme, &a.SkillContent, &a.CreatedAt, &a.UpdatedAt)
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
	} else if !filter.ShowAll {
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
	if filter.AuthorID != "" {
		uid, err := uuid.Parse(filter.AuthorID)
		if err == nil {
			where = append(where, fmt.Sprintf("a.author_id = $%d", argIdx))
			args = append(args, uid)
			argIdx++
		}
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM assets a"
	if whereClause != "" {
		countQuery += " WHERE " + whereClause
	}
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	orderBy := "a.created_at DESC"
	if filter.Sort == "downloads" {
		orderBy = "a.download_count DESC, a.created_at DESC"
	} else if filter.Sort == "rating" {
		orderBy = "a.avg_rating DESC, a.created_at DESC"
	}

	querySQL := fmt.Sprintf(
		`SELECT a.id, a.name, a.type, a.description, a.author_id, u.username, a.status, a.tags, a.current_version, a.rejection_reason, a.download_count, a.avg_rating, a.install_count, a.asset_readme, a.asset_skill_content, a.created_at, a.updated_at
		 FROM assets a JOIN users u ON a.author_id = u.id%s ORDER BY %s LIMIT $%d OFFSET $%d`,
		func() string {
			if whereClause != "" {
				return " WHERE " + whereClause
			}
			return ""
		}(), orderBy, argIdx, argIdx+1)
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
			&a.Status, &a.Tags, &a.CurrentVersion, &a.RejectionReason, &a.DownloadCount, &a.AvgRating, &a.InstallCount, &a.Readme, &a.SkillContent, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
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

func (r *assetRepo) UpdateAvgRating(ctx context.Context, assetID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET avg_rating = COALESCE(
			(SELECT AVG((COALESCE(compatibility,0) + COALESCE(usefulness,0) + COALESCE(security,0)) / 3.0)
			 FROM reviews WHERE asset_id = $1), 0)
		WHERE id = $1`,
		assetID)
	return err
}

func (r *assetRepo) PopularTags(ctx context.Context, limit int) ([]TagCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT unnest(tags) as tag, COUNT(*) as count FROM assets WHERE status = 'approved' GROUP BY tag ORDER BY count DESC LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagCount
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *assetRepo) Update(ctx context.Context, id uuid.UUID, name, description string, tags []string, status model.AssetStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET name = $1, description = $2, tags = $3, status = $4, updated_at = NOW() WHERE id = $5`,
		name, description, tags, status, id)
	return err
}

func (r *assetRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, id)
	return err
}

func (r *assetRepo) GetByName(ctx context.Context, name string) (*model.Asset, error) {
	var a model.Asset
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.name, a.type, a.description, a.author_id, u.username, a.status, a.tags, a.current_version, a.rejection_reason, a.download_count, a.avg_rating, a.install_count, a.asset_readme, a.asset_skill_content, a.created_at, a.updated_at
		 FROM assets a JOIN users u ON a.author_id = u.id WHERE a.name = $1 AND a.status = 'approved'`, name).
		Scan(&a.ID, &a.Name, &a.Type, &a.Description, &a.AuthorID, &a.AuthorName,
			&a.Status, &a.Tags, &a.CurrentVersion, &a.RejectionReason, &a.DownloadCount, &a.AvgRating, &a.InstallCount, &a.Readme, &a.SkillContent, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	return &a, err
}

func (r *assetRepo) IncrementDownloadCount(ctx context.Context, assetID, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`INSERT INTO asset_downloads (asset_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		assetID, userID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 1 {
		_, err = tx.Exec(ctx,
			`UPDATE assets SET download_count = download_count + 1 WHERE id = $1`,
			assetID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *assetRepo) TotalDownloads(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, "SELECT COALESCE(SUM(download_count), 0) FROM assets WHERE status = $1", model.AssetStatusApproved).Scan(&total)
	return total, err
}

func (r *assetRepo) CountByDay(ctx context.Context, days int) ([]DayCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM assets
		 WHERE created_at >= NOW() - $1 * interval '1 day'
		 GROUP BY date
		 ORDER BY date`,
		days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DayCount
	for rows.Next() {
		var dc DayCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return nil, err
		}
		result = append(result, dc)
	}
	return result, rows.Err()
}

func (r *assetRepo) CountDownloadsByDay(ctx context.Context, days int) ([]DayCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT TO_CHAR(downloaded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM asset_downloads
		 WHERE downloaded_at >= NOW() - $1 * interval '1 day'
		 GROUP BY date
		 ORDER BY date`,
		days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DayCount
	for rows.Next() {
		var dc DayCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return nil, err
		}
		result = append(result, dc)
	}
	return result, rows.Err()
}


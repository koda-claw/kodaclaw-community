package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateUsername = errors.New("username already exists")
	ErrDuplicateAPIKey   = errors.New("api key already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	CreateWithGitHub(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*model.User, error)
	GetByGitHubID(ctx context.Context, githubID int64) (*model.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, description *string) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL string) error
	GetByClaimToken(ctx context.Context, token string) (*model.User, error)
	ClaimKodaClawUser(ctx context.Context, kodaclawUserID, humanUserID uuid.UUID) error
	GetClaimedInstances(ctx context.Context, humanUserID uuid.UUID) ([]model.User, error)
	CleanExpiredClaimTokens(ctx context.Context) (int64, error)
	Count(ctx context.Context) (int64, error)
}

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{pool: pool}
}

func RunGitHubMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS github_id BIGINT`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS github_username VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id) WHERE github_id IS NOT NULL`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, claim_token, claim_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		user.ID, user.Username, user.PasswordHash, user.APIKey, user.UserType,
		user.InstanceID, user.DisplayName, user.Description, user.IsAdmin,
		user.ClaimToken, user.ClaimExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "username") {
				return ErrDuplicateUsername
			}
			return ErrDuplicateAPIKey
		}
	}
	return err
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, created_at, updated_at
		 FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, created_at, updated_at
		 FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) UpdateProfile(ctx context.Context, id uuid.UUID, displayName, description *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET display_name = $1, description = $2, updated_at = NOW() WHERE id = $3`,
		displayName, description, id)
	return err
}

func (r *userRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		passwordHash, id)
	return err
}

func (r *userRepo) GetByAPIKey(ctx context.Context, apiKey string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, created_at, updated_at
		 FROM users WHERE api_key = $1`, apiKey).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) GetByGitHubID(ctx context.Context, githubID int64) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, created_at, updated_at
		 FROM users WHERE github_id = $1`, githubID).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`,
		avatarURL, id)
	return err
}

func (r *userRepo) GetByClaimToken(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin,
		        claim_token, claim_expires_at, claimed_by, claimed_at, created_at, updated_at
		 FROM users WHERE claim_token = $1`, token).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin,
			&u.ClaimToken, &u.ClaimExpiresAt, &u.ClaimedBy, &u.ClaimedAt,
			&u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) ClaimKodaClawUser(ctx context.Context, kodaclawUserID, humanUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET claimed_by = $1, claimed_at = NOW(), claim_token = NULL, claim_expires_at = NULL, updated_at = NOW()
		 WHERE id = $2 AND user_type = 'kodaclaw'`,
		humanUserID, kodaclawUserID)
	return err
}

func (r *userRepo) GetClaimedInstances(ctx context.Context, humanUserID uuid.UUID) ([]model.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, username, user_type, instance_id, display_name, description, claimed_by, claimed_at, created_at, updated_at
		 FROM users WHERE claimed_by = $1 AND user_type = 'kodaclaw' ORDER BY claimed_at DESC`, humanUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.UserType, &u.InstanceID, &u.DisplayName,
			&u.Description, &u.ClaimedBy, &u.ClaimedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

func (r *userRepo) CleanExpiredClaimTokens(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET claim_token = NULL, claim_expires_at = NULL, updated_at = NOW()
		 WHERE claim_token IS NOT NULL AND claim_expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func RunClaimMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claim_token VARCHAR(36)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claim_expires_at TIMESTAMP`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_by UUID REFERENCES users(id)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMP`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_claim_token ON users(claim_token) WHERE claim_token IS NOT NULL`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func (r *userRepo) CreateWithGitHub(ctx context.Context, user *model.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, avatar_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		user.ID, user.Username, user.PasswordHash, user.APIKey, user.UserType,
		user.InstanceID, user.DisplayName, user.Description, user.IsAdmin,
		user.GitHubID, user.GitHubUsername, user.AvatarURL)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "username") {
				return ErrDuplicateUsername
			}
			return ErrDuplicateAPIKey
		}
	}
	return err
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

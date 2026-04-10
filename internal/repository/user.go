package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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
	GetByBindCode(ctx context.Context, code string) (*model.User, error)
	BindObserver(ctx context.Context, kodaclawUserID, observerUserID uuid.UUID) error
	GetObservedInstance(ctx context.Context, observerUserID uuid.UUID) ([]model.User, error)
	Count(ctx context.Context) (int64, error)
	CountByDay(ctx context.Context, days int) ([]DayCount, error)
	UpdateResetToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error
	GetUserByResetToken(ctx context.Context, token string) (*model.User, error)
	ClearResetToken(ctx context.Context, userID uuid.UUID) error
	UpdateAPIKey(ctx context.Context, userID uuid.UUID, newKey string) error
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
		`INSERT INTO users (id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, bind_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID, user.Username, user.PasswordHash, user.APIKey, user.UserType,
		user.InstanceID, user.DisplayName, user.Description, user.IsAdmin,
		user.BindCode)
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
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, created_at, updated_at
		 FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.GitHubID, &u.GitHubUsername, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, created_at, updated_at
		 FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.GitHubID, &u.GitHubUsername, &u.CreatedAt, &u.UpdatedAt)
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
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, created_at, updated_at
		 FROM users WHERE api_key = $1`, apiKey).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.GitHubID, &u.GitHubUsername, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) GetByGitHubID(ctx context.Context, githubID int64) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, created_at, updated_at
		 FROM users WHERE github_id = $1`, githubID).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin, &u.GitHubID, &u.GitHubUsername, &u.CreatedAt, &u.UpdatedAt)
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

func (r *userRepo) GetByBindCode(ctx context.Context, code string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin,
		        bind_code, observer_id, bound_at, created_at, updated_at
		 FROM users WHERE bind_code = $1`, code).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin,
			&u.BindCode, &u.ObserverID, &u.BoundAt,
			&u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *userRepo) BindObserver(ctx context.Context, kodaclawUserID, observerUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET observer_id = $1, bound_at = NOW(), bind_code = NULL, updated_at = NOW()
		 WHERE id = $2 AND user_type = 'kodaclaw'`,
		observerUserID, kodaclawUserID)
	return err
}

func (r *userRepo) GetObservedInstance(ctx context.Context, observerUserID uuid.UUID) ([]model.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, username, user_type, instance_id, display_name, description, is_admin, observer_id, bound_at, created_at, updated_at
		 FROM users WHERE observer_id = $1 AND user_type = 'kodaclaw' ORDER BY bound_at DESC`, observerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.UserType, &u.InstanceID, &u.DisplayName,
			&u.Description, &u.IsAdmin, &u.ObserverID, &u.BoundAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
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

func RunAccountRecoveryMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key_reset_token VARCHAR(36)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key_reset_expires TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_users_api_key_reset_token ON users(api_key_reset_token) WHERE api_key_reset_token IS NOT NULL`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func RunBindMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bind_code VARCHAR(36)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS observer_id UUID REFERENCES users(id)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bound_at TIMESTAMP`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_bind_code ON users(bind_code) WHERE bind_code IS NOT NULL`,
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

func (r *userRepo) CountByDay(ctx context.Context, days int) ([]DayCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS date, COUNT(*) AS count
		 FROM users
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


func (r *userRepo) UpdateResetToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET api_key_reset_token = $1, api_key_reset_expires = $2, updated_at = NOW() WHERE id = $3`,
		token, expires, userID)
	return err
}

func (r *userRepo) GetUserByResetToken(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin, github_id, github_username, avatar_url, api_key_reset_token, api_key_reset_expires, created_at, updated_at FROM users WHERE api_key_reset_token = $1`, token).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.APIKey, &u.UserType,
			&u.InstanceID, &u.DisplayName, &u.Description, &u.IsAdmin,
			&u.GitHubID, &u.GitHubUsername, &u.AvatarURL,
			&u.APIKeyResetToken, &u.APIKeyResetExpires,
			&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) ClearResetToken(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET api_key_reset_token = NULL, api_key_reset_expires = NULL, updated_at = NOW() WHERE id = $1`,
		userID)
	return err
}

func (r *userRepo) UpdateAPIKey(ctx context.Context, userID uuid.UUID, newKey string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET api_key = $1, updated_at = NOW() WHERE id = $2`,
		newKey, userID)
	return err
}

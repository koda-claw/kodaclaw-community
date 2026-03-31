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
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*model.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, description *string) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{pool: pool}
}

func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, api_key, user_type, instance_id, display_name, description, is_admin)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.Username, user.PasswordHash, user.APIKey, user.UserType,
		user.InstanceID, user.DisplayName, user.Description, user.IsAdmin)
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

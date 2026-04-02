package model

import (
	"time"

	"github.com/google/uuid"
)

type UserType string

const (
	UserTypeKodaClaw UserType = "kodaclaw"
)

type User struct {
	ID             uuid.UUID  `json:"id"`
	Username       string     `json:"username"`
	PasswordHash   string     `json:"-"`
	APIKey         string     `json:"-"`
	UserType       UserType   `json:"user_type"`
	InstanceID     *string    `json:"instance_id,omitempty"`
	DisplayName    *string    `json:"display_name,omitempty"`
	Description    *string    `json:"description,omitempty"`
	IsAdmin        bool       `json:"is_admin"`
	GitHubID       *int64     `json:"github_id,omitempty" db:"github_id"`
	GitHubUsername *string    `json:"github_username,omitempty" db:"github_username"`
	AvatarURL      *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	BindCode   *string    `json:"-" db:"bind_code"`
	ObserverID *uuid.UUID `json:"observer_id,omitempty" db:"observer_id"`
	BoundAt    *time.Time `json:"bound_at,omitempty" db:"bound_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type RegisterRequest struct {
	Username    string  `json:"username" binding:"required,min=3,max=50"`
	Password    string  `json:"password" binding:"omitempty,min=8,max=50"`
	InstanceID  *string `json:"instance_id"`
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterResponse struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterResponseWithBind struct {
	RegisterResponse
	BindURL string `json:"bind_url,omitempty"`
}

type LoginResponse struct {
	APIKey string `json:"api_key"`
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	UserType    UserType  `json:"user_type"`
	DisplayName *string   `json:"display_name,omitempty"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

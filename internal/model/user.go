package model

import (
	"time"

	"github.com/google/uuid"
)

type UserType string

const (
	UserTypeHuman    UserType = "human"
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
	ClaimToken     *string    `json:"-" db:"claim_token"`
	ClaimExpiresAt *time.Time `json:"-" db:"claim_expires_at"`
	ClaimedBy      *uuid.UUID `json:"claimed_by,omitempty" db:"claimed_by"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type RegisterRequest struct {
	Username    string   `json:"username" binding:"required,min=3,max=50"`
	Password    string   `json:"password" binding:"required,min=8,max=50"`
	UserType    UserType `json:"user_type" binding:"required,oneof=human kodaclaw"`
	InstanceID  *string  `json:"instance_id"`
	DisplayName *string  `json:"display_name"`
	Description *string  `json:"description"`
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

type RegisterResponseWithClaim struct {
	RegisterResponse
	ClaimURL string `json:"claim_url,omitempty"`
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

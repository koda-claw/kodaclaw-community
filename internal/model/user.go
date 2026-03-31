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
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	APIKey       string    `json:"-"`
	UserType     UserType  `json:"user_type"`
	InstanceID   *string   `json:"instance_id,omitempty"`
	DisplayName  *string   `json:"display_name,omitempty"`
	Description  *string   `json:"description,omitempty"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterRequest struct {
	Username    string   `json:"username" binding:"required,min=3,max=50"`
	Password    string   `json:"password" binding:"required,min=8,max=50"`
	UserType    UserType `json:"user_type" binding:"omitempty,oneof=human kodaclaw"`
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

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidSelectToken = errors.New("invalid or expired select token")
	ErrInvalidSessionToken = errors.New("invalid or expired session token")
)

type SessionData struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	IsAdmin  bool      `json:"is_admin"`
}

type sessionDataInternal struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	ExpiresAt string    `json:"expires_at"`
}

// NewRedisClient creates a new Redis client from a URL.
func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	return redis.NewClient(opts), nil
}

// CreateSelectToken generates a select_token and stores it in Redis with 5 min TTL.
func CreateSelectToken(ctx context.Context, rdb *redis.Client, githubID int64) (string, error) {
	token := uuid.New().String()
	key := fmt.Sprintf("select:%s", token)
	err := rdb.Set(ctx, key, fmt.Sprintf("%d", githubID), 5*time.Minute).Err()
	if err != nil {
		return "", fmt.Errorf("failed to create select token: %w", err)
	}
	return token, nil
}

// ValidateSelectToken checks if a select_token is valid and returns the githubID.
func ValidateSelectToken(ctx context.Context, rdb *redis.Client, token string) (int64, error) {
	key := fmt.Sprintf("select:%s", token)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrInvalidSelectToken
		}
		return 0, fmt.Errorf("failed to validate select token: %w", err)
	}
	githubID, err := parseGitHubID(val)
	if err != nil {
		return 0, err
	}
	return githubID, nil
}

// ConsumeSelectToken validates and deletes a select_token atomically (get+delete).
func ConsumeSelectToken(ctx context.Context, rdb *redis.Client, token string) (int64, error) {
	key := fmt.Sprintf("select:%s", token)
	val, err := rdb.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrInvalidSelectToken
		}
		return 0, fmt.Errorf("failed to consume select token: %w", err)
	}
	githubID, err := parseGitHubID(val)
	if err != nil {
		return 0, err
	}
	return githubID, nil
}

func parseGitHubID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		return 0, fmt.Errorf("invalid github_id in redis: %w", err)
	}
	return id, nil
}

// CreateSessionToken generates a session_token and stores session data in Redis with 24h TTL.
func CreateSessionToken(ctx context.Context, rdb *redis.Client, userID uuid.UUID, username string, isAdmin bool) (string, error) {
	token := uuid.New().String()
	key := fmt.Sprintf("session:%s", token)
	expiresAt := time.Now().Add(24 * time.Hour)
	data := sessionDataInternal{
		UserID:    userID,
		Username:  username,
		IsAdmin:   isAdmin,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}
	val, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session data: %w", err)
	}
	err = rdb.Set(ctx, key, val, 24*time.Hour).Err()
	if err != nil {
		return "", fmt.Errorf("failed to create session token: %w", err)
	}
	return token, nil
}

// ValidateSessionToken checks if a session_token is valid and returns the session data.
func ValidateSessionToken(ctx context.Context, rdb *redis.Client, token string) (*SessionData, error) {
	key := fmt.Sprintf("session:%s", token)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidSessionToken
		}
		return nil, fmt.Errorf("failed to validate session token: %w", err)
	}
	var data sessionDataInternal
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	// Check expiration
	expiresAt, err := time.Parse(time.RFC3339, data.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("invalid expires_at format in session: %w", err)
	}
	if time.Now().After(expiresAt) {
		return nil, ErrInvalidSessionToken
	}
	return &SessionData{
		UserID:   data.UserID,
		Username: data.Username,
		IsAdmin:  data.IsAdmin,
	}, nil
}

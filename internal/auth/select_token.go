package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

// InitRedis sets the Redis client for select_token operations.
// It uses an externally-created client (from main.go) to avoid duplicates.
func InitRedis(client *redis.Client) {
	rdb = client
}

// CreateSelectToken generates a select_token, stores it in Redis with 5min TTL.
func CreateSelectToken(ctx context.Context, githubID int64) (string, error) {
	token := uuid.New().String()
	err := rdb.Set(ctx, fmt.Sprintf("select:%s", token), fmt.Sprintf("%d", githubID), 5*time.Minute).Err()
	return token, err
}

// ValidateSelectToken reads a select_token without consuming it.
func ValidateSelectToken(ctx context.Context, token string) (int64, error) {
	key := fmt.Sprintf("select:%s", token)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("invalid or expired select token")
	}
	var githubID int64
	fmt.Sscanf(val, "%d", &githubID)
	return githubID, nil
}

// ConsumeSelectToken validates and deletes a select_token (one-time use).
func ConsumeSelectToken(ctx context.Context, token string) (int64, error) {
	key := fmt.Sprintf("select:%s", token)
	val, err := rdb.GetDel(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("invalid or expired select token")
	}
	var githubID int64
	fmt.Sscanf(val, "%d", &githubID)
	return githubID, nil
}

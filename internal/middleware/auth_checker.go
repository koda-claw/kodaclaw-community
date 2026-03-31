package middleware

import (
	"context"

	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// NewAuthChecker creates an AuthChecker backed by the user repository.
func NewAuthChecker(userRepo repository.UserRepository) AuthChecker {
	return authCheckerFunc(func(ctx context.Context, apiKey string) (string, string, bool, error) {
		user, err := userRepo.GetByAPIKey(ctx, apiKey)
		if err != nil {
			return "", "", false, nil
		}
		return user.ID.String(), string(user.UserType), user.IsAdmin, nil
	})
}

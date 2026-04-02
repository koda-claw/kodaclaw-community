package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// NewAuthChecker creates an AuthChecker backed by the user repository.
func NewAuthChecker(userRepo repository.UserRepository) AuthChecker {
	return authCheckerFunc(func(ctx context.Context, apiKey string) (string, string, bool, error) {
		user, err := userRepo.GetByAPIKey(ctx, apiKey)
		if err != nil {
			return "", "", false, nil
		}
		isAdmin := user.IsAdmin
		// Observer inherits admin from observed KodaClaw instance
		if !isAdmin {
			instances, err := userRepo.GetObservedInstance(ctx, uuid.MustParse(user.ID.String()))
			if err == nil {
				for _, inst := range instances {
					if inst.IsAdmin {
						isAdmin = true
						break
					}
				}
			}
		}
		return user.ID.String(), string(user.UserType), isAdmin, nil
	})
}

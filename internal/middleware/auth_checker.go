package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// NewAuthChecker creates an AuthChecker backed by the user repository.
func NewAuthChecker(userRepo repository.UserRepository) AuthChecker {
	type observedRepo interface {
		GetObservedInstance(ctx context.Context, observerUserID uuid.UUID) ([]interface{ GetIsAdmin() bool }, error)
	}
	return authCheckerFunc(func(ctx context.Context, apiKey string) (string, string, bool, error) {
		user, err := userRepo.GetByAPIKey(ctx, apiKey)
		if err != nil {
			return "", "", false, nil
		}
		isAdmin := user.IsAdmin
		// Observer inherits admin from observed KodaClaw instance
		if !isAdmin {
			if or, ok := userRepo.(observedRepo); ok {
				instances, err := or.GetObservedInstance(ctx, user.ID)
				if err == nil {
					for _, inst := range instances {
						if inst.GetIsAdmin() {
							isAdmin = true
							break
						}
					}
				}
			}
		}
		return user.ID.String(), string(user.UserType), isAdmin, nil
	})
}

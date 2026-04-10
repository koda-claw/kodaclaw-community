package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/auth"
)

const (
	ContextUserID   = "user_id"
	ContextUsername  = "username"
	ContextUserType = "user_type"
	ContextIsAdmin  = "is_admin"
)

// AuthChecker validates an API key and returns user info.
// context is provided by the gin handler for database access.
type AuthChecker interface {
	CheckAPIKey(ctx context.Context, apiKey string) (userID string, userType string, isAdmin bool, err error)
}

// authCheckerFunc adapts a function to the AuthChecker interface.
type authCheckerFunc func(ctx context.Context, apiKey string) (string, string, bool, error)

func (f authCheckerFunc) CheckAPIKey(ctx context.Context, apiKey string) (string, string, bool, error) {
	return f(ctx, apiKey)
}

func AuthMiddleware(checker AuthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Try JWT validation first
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if claims, err := auth.ValidateToken(tokenString); err == nil {
				// JWT validation succeeded
				c.Set(ContextUserID, claims.UserID)
				c.Set(ContextUsername, claims.Username)
				c.Set(ContextIsAdmin, claims.IsAdmin)
				c.Next()
				return
			}
			// JWT validation failed, continue to API key path
		}

		if authHeader == "" {
			RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid Authorization header format, expected: Bearer {api_key}")
			return
		}

		apiKey := strings.TrimSpace(parts[1])
		if apiKey == "" {
			RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "API key is empty")
			return
		}

		userID, userType, isAdmin, err := checker.CheckAPIKey(c.Request.Context(), apiKey)
		if err != nil || userID == "" {
			RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key")
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextUserType, userType)
		c.Set(ContextIsAdmin, isAdmin)
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get(ContextIsAdmin)
		if !exists || !isAdmin.(bool) {
			RespondError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
			return
		}
		c.Next()
	}
}

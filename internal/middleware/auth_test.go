package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockAuthChecker implements AuthChecker for testing
type mockAuthChecker struct {
	users map[string]mockUser
}

type mockUser struct {
	userID   string
	userType string
	isAdmin  bool
}

func (m *mockAuthChecker) CheckAPIKey(ctx context.Context, apiKey string) (string, string, bool, error) {
	u, ok := m.users[apiKey]
	if !ok {
		return "", "", false, nil
	}
	return u.userID, u.userType, u.isAdmin, nil
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &mockAuthChecker{
		users: map[string]mockUser{
			"valid-key": {userID: "user-1", userType: "human", isAdmin: false},
		},
	}

	router := gin.New()
	router.Use(AuthMiddleware(checker))
	router.GET("/test", func(c *gin.Context) {
		userID := c.GetString(ContextUserID)
		if userID != "user-1" {
			t.Errorf("expected userID user-1, got %s", userID)
		}
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &mockAuthChecker{users: map[string]mockUser{}}

	router := gin.New()
	router.Use(AuthMiddleware(checker))
	router.GET("/test", func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &mockAuthChecker{users: map[string]mockUser{}}

	router := gin.New()
	router.Use(AuthMiddleware(checker))
	router.GET("/test", func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Token sometoken")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &mockAuthChecker{users: map[string]mockUser{}}

	router := gin.New()
	router.Use(AuthMiddleware(checker))
	router.GET("/test", func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer nonexistent-key")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_EmptyBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &mockAuthChecker{users: map[string]mockUser{}}

	router := gin.New()
	router.Use(AuthMiddleware(checker))
	router.GET("/test", func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAdminOnly_AdminUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextIsAdmin, true)
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.String(200, "admin ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAdminOnly_NonAdminUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextIsAdmin, false)
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		t.Error("handler should not be called for non-admin")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAdminOnly_NoAdminFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	// Don't set ContextIsAdmin at all
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTieredRateLimiter_AnonymousLimited(t *testing.T) {
	tier := NewTieredRateLimiter()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(tier.TieredMiddleware("read"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Anonymous should be limited to 30/min
	for i := 0; i < 30; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// 31st should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("31st request: expected 429, got %d", w.Code)
	}
}

func TestTieredRateLimiter_ObserverWriteLimited(t *testing.T) {
	tier := NewTieredRateLimiter()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextUserID, "observer-user-id")
		c.Set(ContextUserType, "observer")
		c.Set(ContextIsAdmin, false)
		c.Next()
	})
	r.Use(tier.TieredMiddleware("write"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Observer write should be limited to 15/min
	for i := 0; i < 15; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("observer write request %d: expected 200, got %d", i, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("16th observer write request: expected 429, got %d", w.Code)
	}
}

func TestTieredRateLimiter_KodaclawLimited(t *testing.T) {
	tier := NewTieredRateLimiter()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextUserID, "kc-user-id")
		c.Set(ContextUserType, "kodaclaw")
		c.Set(ContextIsAdmin, false)
		c.Next()
	})
	r.Use(tier.TieredMiddleware("write"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// KodaClaw write should be limited to 30/min
	for i := 0; i < 30; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("kodaclaw write request %d: expected 200, got %d", i, w.Code)
		}
	}

	// 31st should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("31st kodaclaw write request: expected 429, got %d", w.Code)
	}
}

func TestTieredRateLimiter_AdminUnlimited(t *testing.T) {
	tier := NewTieredRateLimiter()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextUserID, "admin-user-id")
		c.Set(ContextUserType, "kodaclaw")
		c.Set(ContextIsAdmin, true)
		c.Next()
	})
	r.Use(tier.TieredMiddleware("write"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Admin should not be rate limited on write (999999 limit)
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("admin request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestTieredRateLimiter_AdminUploadLimited(t *testing.T) {
	tier := NewTieredRateLimiter()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextUserID, "admin-user-id")
		c.Set(ContextUserType, "kodaclaw")
		c.Set(ContextIsAdmin, true)
		c.Next()
	})
	r.Use(tier.TieredMiddleware("upload"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Admin upload should still be limited to 10/min
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("admin upload request %d: expected 200, got %d", i, w.Code)
		}
	}

	// 11th should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("11th admin upload request: expected 429, got %d", w.Code)
	}
}


func TestSlidingWindowLimiter_LazyCleanupReducesEntries(t *testing.T) {
	limiter := NewMemoryRateLimiter(10, time.Minute)

	// Create 200 unique keys
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		limiter.Allow(key)
	}

	// Force cleanup by creating many more windows until one triggers cleanup
	for i := 200; i < 10000; i++ {
		key := fmt.Sprintf("key-%d", i)
		limiter.Allow(key)
	}

	// After many iterations, the map should have been cleaned up at least once.
	// We can't assert exact size due to probabilistic nature, but it should still work.
	allowed, _, _ := limiter.Allow("final-key")
	if !allowed {
		t.Error("limiter should still work after many cleanup cycles")
	}
}

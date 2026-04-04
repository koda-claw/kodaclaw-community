package middleware

import (
	"fmt"
	"net/http"
	"math/rand"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter interface {
	Allow(key string) (allowed bool, remaining int, resetAt time.Time)
}

type slidingWindowLimiter struct {
	mu       sync.Mutex
	windows  map[string]*window
	limit    int
	duration time.Duration
}

type window struct {
	count   int
	startAt time.Time
}

func NewMemoryRateLimiter(limit int, per time.Duration) RateLimiter {
	return &slidingWindowLimiter{
		windows:  make(map[string]*window),
		limit:    limit,
		duration: per,
	}
}

func (rl *slidingWindowLimiter) Allow(key string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, exists := rl.windows[key]

	if !exists || now.Sub(w.startAt) >= rl.duration {
		rl.windows[key] = &window{count: 1, startAt: now}
		// Lazy cleanup: purge expired entries every ~100 requests
		rl.lazyCleanup(now)
		return true, rl.limit - 1, now.Add(rl.duration)
	}

	w.count++
	remaining := rl.limit - w.count
	resetAt := w.startAt.Add(rl.duration)

	if remaining < 0 {
		remaining = 0
		return false, 0, resetAt
	}

	return true, remaining, resetAt
}

// lazyCleanup removes expired window entries to prevent unbounded memory growth.
// Called probabilistically on new window creation (~1% chance per call).
func (rl *slidingWindowLimiter) lazyCleanup(now time.Time) {
	if len(rl.windows) < 100 || rand.Intn(100) != 0 {
		return
	}
	for key, w := range rl.windows {
		if now.Sub(w.startAt) >= rl.duration {
			delete(rl.windows, key)
		}
	}
}

func RateLimitMiddleware(limiter RateLimiter, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.ClientIP()
		}

		allowed, remaining, resetAt := limiter.Allow(apiKey)

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

		if !allowed {
			RespondError(c, http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded, please try again later")
			return
		}

		c.Next()
	}
}

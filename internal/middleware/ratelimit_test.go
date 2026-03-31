package middleware

import (
	"testing"
	"time"
)

func TestMemoryRateLimiter_AllowWithinLimit(t *testing.T) {
	limiter := NewMemoryRateLimiter(3, time.Minute)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		allowed, remaining, _ := limiter.Allow("user1")
		if !allowed {
			t.Errorf("request %d: expected allowed, got denied", i)
		}
		expected := 2 - i
		if remaining != expected {
			t.Errorf("request %d: expected remaining %d, got %d", i, expected, remaining)
		}
	}

	// 4th request should be denied
	allowed, remaining, _ := limiter.Allow("user1")
	if allowed {
		t.Error("expected denied after limit exceeded, got allowed")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0 when denied, got %d", remaining)
	}
}

func TestMemoryRateLimiter_DifferentKeys(t *testing.T) {
	limiter := NewMemoryRateLimiter(2, time.Minute)

	// user1 uses both slots
	limiter.Allow("user1")
	limiter.Allow("user1")
	allowed, _, _ := limiter.Allow("user1")
	if allowed {
		t.Error("user1 should be rate limited")
	}

	// user2 should still have its own quota
	allowed, _, _ = limiter.Allow("user2")
	if !allowed {
		t.Error("user2 should not be affected by user1's limit")
	}
}

func TestMemoryRateLimiter_WindowReset(t *testing.T) {
	limiter := NewMemoryRateLimiter(1, 100*time.Millisecond)

	allowed, remaining, _ := limiter.Allow("user1")
	if !allowed {
		t.Error("first request should be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}

	// Immediately denied
	allowed, _, _ = limiter.Allow("user1")
	if allowed {
		t.Error("second request should be denied")
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)
	allowed, remaining, resetAt := limiter.Allow("user1")
	if !allowed {
		t.Error("request after window reset should be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}
	now := time.Now()
	if resetAt.Before(now) {
		t.Error("resetAt should be in the future")
	}
}

func TestMemoryRateLimiter_ResetTime(t *testing.T) {
	limiter := NewMemoryRateLimiter(5, 10*time.Second)

	before := time.Now()
	_, _, resetAt := limiter.Allow("user1")
	after := time.Now()

	// resetAt should be roughly 10 seconds from now
	expectedMin := before.Add(10 * time.Second)
	expectedMax := after.Add(10 * time.Second)

	if resetAt.Before(expectedMin) || resetAt.After(expectedMax) {
		t.Errorf("resetAt %v should be between %v and %v", resetAt, expectedMin, expectedMax)
	}
}

package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// rateTier holds per-operation limits for a user role.
type rateTier struct {
	read     int
	write    int
	upload   int
	download int
}

// TieredRateLimiter applies different rate limits based on user role.
type TieredRateLimiter struct {
	admin    map[string]RateLimiter // operation -> limiter
	kodaclaw map[string]RateLimiter
	observer map[string]RateLimiter
	anon     map[string]RateLimiter
}

// tierLimits defines rate limits per operation per role.
var tierLimits = map[string]rateTier{
	"admin": {
		read:     999999, // unlimited
		write:    999999,
		upload:   10,
		download: 999999,
	},
	"kodaclaw": {
		read:     100,
		write:    30,
		upload:   10,
		download: 30,
	},
	"observer": {
		read:     60,
		write:    15,
		upload:   5,
		download: 20,
	},
	"anonymous": {
		read:     30,
		write:    5,
		upload:   5,
		download: 10,
	},
}

// NewTieredRateLimiter creates a tiered rate limiter with pre-configured limits.
func NewTieredRateLimiter() *TieredRateLimiter {
	t := &TieredRateLimiter{
		admin:    make(map[string]RateLimiter),
		kodaclaw: make(map[string]RateLimiter),
		observer: make(map[string]RateLimiter),
		anon:     make(map[string]RateLimiter),
	}
	for role, limits := range tierLimits {
		m := t.getTierMap(role)
		for op, limit := range map[string]int{
			"read": limits.read, "write": limits.write,
			"upload": limits.upload, "download": limits.download,
		} {
			m[op] = NewMemoryRateLimiter(limit, time.Minute)
		}
	}
	return t
}

func (t *TieredRateLimiter) getTierMap(role string) map[string]RateLimiter {
	switch role {
	case "admin":
		return t.admin
	case "kodaclaw":
		return t.kodaclaw
	case "observer":
		return t.observer
	default:
		return t.anon
	}
}

func (t *TieredRateLimiter) getTier(c *gin.Context) string {
	isAdminVal, exists := c.Get(ContextIsAdmin)
	if b, ok := isAdminVal.(bool); exists && ok && b {
		return "admin"
	}
	userTypeVal, exists := c.Get(ContextUserType)
	if exists && userTypeVal == "kodaclaw" {
		return "kodaclaw"
	}
	// Has auth (userID set by AuthMiddleware) but not kodaclaw -> observer
	userIDVal, exists := c.Get(ContextUserID)
	if exists && userIDVal != nil && userIDVal != "" {
		return "observer"
	}
	return "anonymous"
}

// TieredMiddleware returns a gin middleware that applies role-based rate limiting.
// operation is one of: "read", "write", "upload", "download".
func (t *TieredRateLimiter) TieredMiddleware(operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tier := t.getTier(c)
		limiter := t.getTierMap(tier)[operation]

		if limiter == nil {
			log.Printf("WARN: unknown rate limit operation: %s", operation)
			c.Next()
			return
		}



		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.ClientIP()
		}

		allowed, remaining, resetAt := limiter.Allow(apiKey)

		// Get the effective limit for headers
		var effectiveLimit int
		if l, ok := tierLimits[tier]; ok {
			switch operation {
			case "read":
				effectiveLimit = l.read
			case "write":
				effectiveLimit = l.write
			case "upload":
				effectiveLimit = l.upload
			case "download":
				effectiveLimit = l.download
			}
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", effectiveLimit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

		if !allowed {
			RespondError(c, http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded, please try again later")
			return
		}

		c.Next()
	}
}

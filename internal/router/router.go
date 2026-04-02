package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/handler"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

func Setup(
	engine *gin.Engine,
	authH *handler.AuthHandler,
	assetH *handler.AssetHandler,
	reviewH *handler.ReviewHandler,
	adminH *handler.AdminHandler,
	userH *handler.UserHandler,
	userRepo repository.UserRepository,
	readLimiter, writeLimiter, uploadLimiter middleware.RateLimiter,
	publicH *handler.PublicHandler,
	githubH *handler.GitHubHandler,
	claimH *handler.ClaimHandler,
	relayH *handler.RelayHandler,
	webhookH *handler.WebhookHandler,
) {
	engine.Use(gin.Recovery())
	engine.Use(middleware.ErrorHandler())

	// Static frontend
	engine.StaticFile("/", "./internal/static/index.html")
	engine.Static("/css", "./internal/static/css")
	engine.Static("/js", "./internal/static/js")
	engine.StaticFile("/openapi.yaml", "./docs/openapi.yaml")

	// 认领页面（无需认证，直接返回 HTML）
	engine.GET("/claim", claimH.GetClaimPage)

	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": "0.3.0"})
	})

	// Bootstrap entry: /skill.md returns koda-community SKILL.md content
	engine.GET("/skill.md", publicH.BootstrapSkill)

	v1 := engine.Group("/api/v1")

	// Auth (no auth required, write rate limit)
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	{
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
		authGroup.GET("/github", githubH.GetAuthURL)
		authGroup.GET("/github/callback", githubH.Callback)
	}

	// Create auth checker
	checker := middleware.NewAuthChecker(userRepo)

	// Independent rate limiters for upload and download
	downloadLimiter := middleware.NewMemoryRateLimiter(30, time.Minute)

	// Auth endpoints that require authentication
	authWriteGroup := v1.Group("/auth")
	authWriteGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	authWriteGroup.Use(middleware.AuthMiddleware(checker))
	{
		authWriteGroup.PATCH("/password", authH.ChangePassword)
	}

	// Read endpoints (GET)
	readGroup := v1.Group("")
	readGroup.Use(middleware.RateLimitMiddleware(readLimiter, 100))
	readGroup.Use(middleware.AuthMiddleware(checker))
	{
		readGroup.GET("/tags/popular", assetH.PopularTags)
		readGroup.GET("/assets", assetH.List)
		readGroup.GET("/assets/:id", assetH.GetByID)
		readGroup.GET("/assets/:id/versions", assetH.ListVersions)
		readGroup.GET("/assets/:id/reviews", reviewH.List)
		readGroup.GET("/assets/:id/dependencies", assetH.ListDependencies)
		readGroup.GET("/users/me", userH.GetMe)
		readGroup.GET("/users/me/favorites", userH.ListFavorites)
		readGroup.GET("/users/me/notifications", userH.ListNotifications)
		readGroup.GET("/users/me/instances", claimH.GetClaimedInstances)
		readGroup.GET("/users/:id", userH.GetByID)
		readGroup.GET("/users/:id/assets", userH.ListAssets)
	}

	// Download endpoint (30/min)
	downloadGroup := v1.Group("")
	downloadGroup.Use(middleware.RateLimitMiddleware(downloadLimiter, 30))
	downloadGroup.Use(middleware.AuthMiddleware(checker))
	{
		downloadGroup.GET("/assets/:id/download", assetH.Download)
	}

	// Upload endpoint (5/min)
	uploadGroup := v1.Group("")
	uploadGroup.Use(middleware.RateLimitMiddleware(uploadLimiter, 5))
	uploadGroup.Use(middleware.AuthMiddleware(checker))
	{
		uploadGroup.POST("/assets", assetH.Create)
	}

	// Write endpoints (POST)
	writeGroup := v1.Group("")
	writeGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	writeGroup.Use(middleware.AuthMiddleware(checker))
	{
		writeGroup.POST("/assets/:id/favorite", assetH.ToggleFavorite)
		writeGroup.POST("/assets/:id/reviews", reviewH.Create)
		writeGroup.POST("/assets/:id/versions", assetH.UploadVersion)
		writeGroup.POST("/assets/:id/dependencies", assetH.AddDependency)
		writeGroup.POST("/assets/:id/install", assetH.InstallAsset)
		writeGroup.PATCH("/assets/:id/versions/current", assetH.SetCurrentVersion)
		writeGroup.PATCH("/assets/:id", assetH.Update)
		writeGroup.DELETE("/assets/:id", assetH.Delete)
		writeGroup.DELETE("/assets/:id/dependencies/:dep_id", assetH.DeleteDependency)
		writeGroup.PATCH("/users/me", userH.UpdateProfile)
		writeGroup.PATCH("/users/me/notifications/read-all", userH.MarkAllNotificationsRead)
		writeGroup.PATCH("/users/me/notifications/:id", userH.MarkNotificationRead)
	}

	// Admin endpoints
	adminGroup := v1.Group("/admin")
	adminGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	adminGroup.Use(middleware.AuthMiddleware(checker))
	adminGroup.Use(middleware.AdminOnly())
	{
		adminGroup.GET("/assets", adminH.ListAssets)
		adminGroup.POST("/assets/:id/approve", adminH.Approve)
		adminGroup.POST("/assets/:id/reject", adminH.Reject)
		adminGroup.POST("/versions/:id/approve", adminH.ApproveVersion)
		adminGroup.POST("/versions/:id/reject", adminH.RejectVersion)
		adminGroup.GET("/versions/pending", adminH.ListPendingVersions)
		adminGroup.POST("/cleanup-orphans", adminH.CleanupOrphans)
		adminGroup.GET("/dashboard/stats", adminH.DashboardStats)
		adminGroup.GET("/dashboard/trends", adminH.DashboardTrends)
		adminGroup.GET("/dashboard/recent-reviews", adminH.RecentReviews)
	}

	// Public endpoints (no auth required, rate limited)
	publicGroup := v1.Group("/public")
	publicGroup.Use(middleware.RateLimitMiddleware(readLimiter, 100))
	{
		publicGroup.GET("/skills", publicH.ListSkills)
		publicGroup.GET("/skills/:name", publicH.GetSkill)
		publicGroup.GET("/skills/:name/SKILL.md", publicH.GetSkillContent)
		publicGroup.GET("/skills/:name/download", publicH.DownloadSkill)
		publicGroup.GET("skills/download/:id", publicH.DownloadSkillByID)
		publicGroup.GET("reviews/:id", publicH.ListReviews)
		publicGroup.GET("/skills-by-id/:id/versions", publicH.ListAssetVersions)
		publicGroup.POST("/claim", claimH.Claim)
		publicGroup.GET("/stats", publicH.Stats)
		publicGroup.GET("/users/:username", publicH.UserProfile)
	}

	if relayH != nil {
		// Relay instance management (auth required)
		relayGroup := v1.Group("/relay")
		relayGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
		relayGroup.Use(middleware.AuthMiddleware(checker))
		{
			relayGroup.POST("/instances", relayH.CreateInstance)
			relayGroup.GET("/instances", relayH.ListInstances)
			relayGroup.DELETE("/instances/:id", relayH.DeleteInstance)
			relayGroup.POST("/instances/test-connection", relayH.TestConnection)
			relayGroup.POST("/instances/:id/regenerate-secret", relayH.RegenerateSecret)
		relayGroup.POST("/instances/:id/regenerate-webhook-secret", relayH.RegenerateWebhookSecret)
			relayGroup.GET("/instances/:id/keys", relayH.ListKeys)
			relayGroup.POST("/instances/:id/keys", relayH.CreateKey)
			relayGroup.DELETE("/instances/:id/keys/:keyId", relayH.DeleteKey)
			relayGroup.PATCH("/instances/:id/keys/:keyId", relayH.ToggleKey)
			if webhookH != nil {
				relayGroup.POST("/instances/:id/test-webhook", webhookH.TestWebhook)
			}
		}

		// WebSocket relay endpoint (public — auth is in the WS protocol)
		engine.GET("/ws/relay", relayH.ServeWS)
	}

	if webhookH != nil {
		// Incoming webhook endpoint (public — no HTTP auth)
		v1.POST("/webhook/incoming/:instanceId", webhookH.IncomingWebhook)
	}
}

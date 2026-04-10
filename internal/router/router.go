package router

import (
	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/handler"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

func Setup(
	engine *gin.Engine,
	version string,
	authH *handler.AuthHandler,
	assetH *handler.AssetHandler,
	reviewH *handler.ReviewHandler,
	adminH *handler.AdminHandler,
	userH *handler.UserHandler,
	userRepo repository.UserRepository,
	readLimiter, writeLimiter middleware.RateLimiter,
	publicH *handler.PublicHandler,
	githubH *handler.GitHubHandler,
	bindH *handler.BindHandler,
	resetKeyH *handler.ResetKeyHandler,
	relayH *handler.RelayHandler,
	webhookH *handler.WebhookHandler,
) {
	engine.Use(middleware.Recovery())
	engine.Use(middleware.RequestLogger())
	engine.Use(middleware.ErrorHandler())

	// Static frontend
	// Bind page (must be before StaticFile to avoid route conflict)
	engine.GET("bind", bindH.GetBindPage)
	engine.GET("/bind-error", githubH.BindErrorPage)
	engine.GET("/claim", func(c *gin.Context) { c.Redirect(302, "/bind?token="+c.Query("token")) })

	// Static frontend
	engine.StaticFile("/", "./internal/static/index.html")
	engine.Static("/css", "./internal/static/css")
	engine.Static("/js", "./internal/static/js")
	engine.StaticFile("/openapi.yaml", "./docs/openapi.yaml")

	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": version})
	})

	// Bootstrap entry: /skill.md returns koda-community SKILL.md content
	engine.GET("/skill.md", publicH.BootstrapSkill)

	v1 := engine.Group("/api/v1")

	// Create auth checker
	checker := middleware.NewAuthChecker(userRepo)

	// Create tiered rate limiter
	tieredLimiter := middleware.NewTieredRateLimiter()

	// Auth (no auth required, use anonymous write rate limit)
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 5))
	{
		authGroup.POST("register", authH.Register)
		authGroup.POST("login", authH.Login)
		authGroup.GET("github", githubH.GetAuthURL)
		authGroup.GET("github/callback", githubH.Callback)
		authGroup.POST("reset-key/request", resetKeyH.ResetKeyRequest)
		authGroup.POST("reset-key/confirm", resetKeyH.ResetKeyConfirm)
		authGroup.GET("check-github/:username", resetKeyH.CheckGitHubByUsername)
		instanceSelectH := handler.NewInstanceSelectHandler(userRepo)
		authGroup.GET("instances", instanceSelectH.ListInstances)
		authGroup.POST("instance/select", instanceSelectH.SelectInstance)
	}

	// Auth endpoints that require authentication (auth first, then tiered rate limit)
	authWriteGroup := v1.Group("/auth")
	authWriteGroup.Use(middleware.AuthMiddleware(checker))
	authWriteGroup.Use(tieredLimiter.TieredMiddleware("write"))
	{
		authWriteGroup.PATCH("password", authH.ChangePassword)
		authWriteGroup.POST("reset-key", resetKeyH.ResetKeyDirect)
	}

	// Read endpoints (auth first, then tiered rate limit)
	readGroup := v1.Group("")
	readGroup.Use(middleware.AuthMiddleware(checker))
	readGroup.Use(tieredLimiter.TieredMiddleware("read"))
	{
		readGroup.GET("tags/popular", assetH.PopularTags)
		readGroup.GET("assets", assetH.List)
		readGroup.GET("assets/:id", assetH.GetByID)
		readGroup.GET("assets/:id/versions", assetH.ListVersions)
		readGroup.GET("assets/:id/reviews", reviewH.List)
		readGroup.GET("assets/:id/dependencies", assetH.ListDependencies)
		readGroup.GET("users/me", userH.GetMe)
		readGroup.GET("users/me/github-status", resetKeyH.GitHubStatus)
		readGroup.GET("users/me/favorites", userH.ListFavorites)
		readGroup.GET("users/me/notifications", userH.ListNotifications)
		readGroup.GET("users/me/observed", bindH.GetObservedInstance)
		readGroup.GET("users/me/instances", func(c *gin.Context) { c.Redirect(302, "/api/v1/users/me/observed") })
		readGroup.GET("users/:id", userH.GetByID)
		readGroup.GET("users/:id/assets", userH.ListAssets)
	}

	// Download endpoint (auth first, then tiered rate limit)
	downloadGroup := v1.Group("")
	downloadGroup.Use(middleware.AuthMiddleware(checker))
	downloadGroup.Use(tieredLimiter.TieredMiddleware("download"))
	{
		downloadGroup.GET("assets/:id/download", assetH.Download)
	}

	// Upload endpoint (auth first, then tiered rate limit)
	uploadGroup := v1.Group("")
	uploadGroup.Use(middleware.AuthMiddleware(checker))
	uploadGroup.Use(tieredLimiter.TieredMiddleware("upload"))
	{
		uploadGroup.POST("assets", assetH.Create)
	}

	// Write endpoints (auth first, then tiered rate limit)
	writeGroup := v1.Group("")
	writeGroup.Use(middleware.AuthMiddleware(checker))
	writeGroup.Use(tieredLimiter.TieredMiddleware("write"))
	{
		writeGroup.POST("assets/:id/favorite", assetH.ToggleFavorite)
		writeGroup.POST("assets/:id/reviews", reviewH.Create)
		writeGroup.POST("assets/:id/versions", assetH.UploadVersion)
		writeGroup.POST("assets/:id/dependencies", assetH.AddDependency)
		writeGroup.POST("assets/:id/install", assetH.InstallAsset)
		writeGroup.PATCH("assets/:id/versions/current", assetH.SetCurrentVersion)
		writeGroup.PATCH("assets/:id", assetH.Update)
		writeGroup.DELETE("assets/:id", assetH.Delete)
		writeGroup.DELETE("assets/:id/dependencies/:dep_id", assetH.DeleteDependency)
		writeGroup.PATCH("users/me", userH.UpdateProfile)
		writeGroup.PATCH("users/me/notifications/read-all", userH.MarkAllNotificationsRead)
		writeGroup.PATCH("users/me/notifications/:id", userH.MarkNotificationRead)
	}

	// Admin read endpoints (auth first, admin-only, read tiered rate limit)
	adminReadGroup := v1.Group("/admin")
	adminReadGroup.Use(middleware.AuthMiddleware(checker))
	adminReadGroup.Use(middleware.AdminOnly())
	adminReadGroup.Use(tieredLimiter.TieredMiddleware("read"))
	{
		adminReadGroup.GET("assets", adminH.ListAssets)
		adminReadGroup.GET("versions/pending", adminH.ListPendingVersions)
		adminReadGroup.GET("dashboard/stats", adminH.DashboardStats)
		adminReadGroup.GET("dashboard/trends", adminH.DashboardTrends)
		adminReadGroup.GET("dashboard/recent-reviews", adminH.RecentReviews)
		adminReadGroup.GET("stats", adminH.Stats)
		adminReadGroup.GET("audit", adminH.AuditLogs)
	}

	// Admin write endpoints (auth first, admin-only, write tiered rate limit)
	adminWriteGroup := v1.Group("/admin")
	adminWriteGroup.Use(middleware.AuthMiddleware(checker))
	adminWriteGroup.Use(middleware.AdminOnly())
	adminWriteGroup.Use(tieredLimiter.TieredMiddleware("write"))
	{
		adminWriteGroup.POST("assets/:id/approve", adminH.Approve)
		adminWriteGroup.POST("assets/:id/reject", adminH.Reject)
		adminWriteGroup.POST("versions/:id/approve", adminH.ApproveVersion)
		adminWriteGroup.POST("versions/:id/reject", adminH.RejectVersion)
		adminWriteGroup.POST("cleanup-orphans", adminH.CleanupOrphans)
	}

	// Public read endpoints (no auth, anonymous read rate limit)
	publicReadGroup := v1.Group("/public")
	publicReadGroup.Use(middleware.RateLimitMiddleware(readLimiter, 30))
	{
		publicReadGroup.GET("/skills", publicH.ListSkills)
		publicReadGroup.GET("/skills/:name", publicH.GetSkill)
		publicReadGroup.GET("/skills/:name/SKILL.md", publicH.GetSkillContent)
		publicReadGroup.GET("/skills/:name/download", publicH.DownloadSkill)
		publicReadGroup.GET("skills/download/:id", publicH.DownloadSkillByID)
		publicReadGroup.GET("/reviews/:id", publicH.ListReviews)
		publicReadGroup.GET("/skills-by-id/:id/versions", publicH.ListAssetVersions)
		publicReadGroup.GET("/stats", publicH.Stats)
		publicReadGroup.GET("/users/:username", publicH.UserProfile)
	}

	// Public write endpoints (no auth, anonymous write rate limit)
	publicWriteGroup := v1.Group("/public")
	publicWriteGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 5))
	{
		publicWriteGroup.POST("/bind", bindH.Bind)
		publicWriteGroup.POST("/claim", func(c *gin.Context) { c.Redirect(302, "/api/v1/public/bind") })
	}

	if relayH != nil {
		// Relay read endpoints (auth first, read tiered rate limit)
		relayReadGroup := v1.Group("/relay")
		relayReadGroup.Use(middleware.AuthMiddleware(checker))
		relayReadGroup.Use(tieredLimiter.TieredMiddleware("read"))
		{
			relayReadGroup.GET("/instances", relayH.ListInstances)
			relayReadGroup.GET("/instances/:id/keys", relayH.ListKeys)
		}

		// Relay write endpoints (auth first, write tiered rate limit)
		relayWriteGroup := v1.Group("/relay")
		relayWriteGroup.Use(middleware.AuthMiddleware(checker))
		relayWriteGroup.Use(tieredLimiter.TieredMiddleware("write"))
		{
			relayWriteGroup.POST("/instances", relayH.CreateInstance)
			relayWriteGroup.DELETE("/instances/:id", relayH.DeleteInstance)
			relayWriteGroup.POST("/instances/test-connection", relayH.TestConnection)
			relayWriteGroup.POST("/instances/:id/regenerate-secret", relayH.RegenerateSecret)
			relayWriteGroup.POST("/instances/:id/regenerate-webhook-secret", relayH.RegenerateWebhookSecret)
			relayWriteGroup.POST("/instances/:id/keys", relayH.CreateKey)
			relayWriteGroup.DELETE("/instances/:id/keys/:keyId", relayH.DeleteKey)
			relayWriteGroup.PATCH("/instances/:id/keys/:keyId", relayH.ToggleKey)
			if webhookH != nil {
				relayWriteGroup.POST("/instances/:id/test-webhook", webhookH.TestWebhook)
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

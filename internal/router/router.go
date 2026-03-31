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
) {
	engine.Use(gin.Recovery())
	engine.Use(middleware.ErrorHandler())

	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": "0.1.0"})
	})

	v1 := engine.Group("/api/v1")

	// Auth (no auth required, write rate limit)
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	{
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
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
	}
}

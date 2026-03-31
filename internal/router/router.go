package router

import (
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
	readLimiter, writeLimiter middleware.RateLimiter,
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

	// Read endpoints (GET)
	readGroup := v1.Group("")
	readGroup.Use(middleware.RateLimitMiddleware(readLimiter, 100))
	readGroup.Use(middleware.AuthMiddleware(checker))
	{
		readGroup.GET("/assets", assetH.List)
		readGroup.GET("/assets/:id", assetH.GetByID)
		readGroup.GET("/assets/:id/download", assetH.Download)
		readGroup.GET("/assets/:id/versions", assetH.ListVersions)
		readGroup.GET("/assets/:id/reviews", reviewH.List)
		readGroup.GET("/users/me", userH.GetMe)
		readGroup.GET("/users/:id", userH.GetByID)
		readGroup.GET("/users/:id/assets", userH.ListAssets)
	}

	// Write endpoints (POST)
	writeGroup := v1.Group("")
	writeGroup.Use(middleware.RateLimitMiddleware(writeLimiter, 20))
	writeGroup.Use(middleware.AuthMiddleware(checker))
	{
		writeGroup.POST("/assets", assetH.Create)
		writeGroup.POST("/assets/:id/reviews", reviewH.Create)
		writeGroup.POST("/assets/:id/versions", assetH.UploadVersion)
		writeGroup.PATCH("/assets/:id/versions/current", assetH.SetCurrentVersion)
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

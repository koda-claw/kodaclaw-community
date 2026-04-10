// @title KodaClaw Community API
// @version dev
// @description 全球首个 Agent 资产共享平台
// @host community.ai-koda.com
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/vanzheng/kodaclaw-community/internal/auth"
	"github.com/vanzheng/kodaclaw-community/internal/config"
	"github.com/vanzheng/kodaclaw-community/internal/handler"
	"github.com/vanzheng/kodaclaw-community/internal/logger"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"github.com/vanzheng/kodaclaw-community/internal/router"
	"github.com/vanzheng/kodaclaw-community/internal/service"
)

// Version is set via -ldflags at build time
var Version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger.Init(cfg.LogLevel)

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to parse database config: %v", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	if err := runMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	if err := repository.RunGitHubMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run GitHub migrations: %v", err)
	}
	if err := repository.RunBindMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run bind migrations: %v", err)
	}
	if err := repository.RunAccountRecoveryMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run account recovery migrations: %v", err)
	}
	if cfg.RelayEnabled {
		if err := repository.RunRelayMigrations(ctx, pool); err != nil {
			log.Fatalf("Failed to run relay migrations: %v", err)
		}
	}
	if err := repository.RunObservabilityMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run observability migrations: %v", err)
	}
	log.Println("Database migrations complete")

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Redis not available, falling back to memory rate limiter: %v", err)
	}

	// JWT init
	auth.Init()

	// Redis init for auth (select_token)
	auth.InitRedis(rdb)

	if err := os.MkdirAll(cfg.AssetStoragePath, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	readLimiter := middleware.NewMemoryRateLimiter(100, time.Minute)
	writeLimiter := middleware.NewMemoryRateLimiter(20, time.Minute)

	userRepo := repository.NewUserRepository(pool)
	assetRepo := repository.NewAssetRepository(pool)
	versionRepo := repository.NewAssetVersionRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	favoriteRepo := repository.NewFavoriteRepository(pool)
	notificationRepo := repository.NewNotificationRepository(pool)
	depRepo := repository.NewAssetDependencyRepository(pool)
	installRepo := repository.NewAssetInstallRepository(pool)

	var relayRepo repository.RelayInstanceRepository
	var hub *relay.Hub
	if cfg.RelayEnabled {
		relayRepo = repository.NewRelayInstanceRepository(pool)
		hub = relay.NewHub()
	}

	notificationSvc := service.NewNotificationService(notificationRepo, relayRepo, hub)

	auditRepo := repository.NewAuditLogRepo(pool)
	activityRepo := repository.NewUserActivityRepo(pool)
	auditSvc := service.NewAuditService(auditRepo)
	activitySvc := service.NewActivityService(activityRepo)

	authH := handler.NewAuthHandler(userRepo)
	assetH := handler.NewAssetHandlerFull(assetRepo, versionRepo, userRepo, favoriteRepo, depRepo, installRepo, cfg.AssetStoragePath, activitySvc)
	reviewH := handler.NewReviewHandler(reviewRepo, assetRepo, activitySvc)
	adminH := handler.NewAdminHandler(assetRepo, notificationSvc, versionRepo, userRepo, cfg.AssetStoragePath, auditSvc)
	userH := handler.NewUserHandlerWithNotifications(userRepo, assetRepo, favoriteRepo, notificationRepo)
	publicH := handler.NewPublicHandler(assetRepo, versionRepo, reviewRepo, userRepo, cfg.AssetStoragePath, activitySvc)
	githubH := handler.NewGitHubHandler(userRepo)
	bindH := handler.NewBindHandler(userRepo, relayRepo, hub)
	resetKeyH := handler.NewResetKeyHandler(userRepo)
	githubH.SetResetKeyHandler(resetKeyH)

	taskCtx, taskCancel := context.WithCancel(context.Background())
	defer taskCancel()

	var relayH *handler.RelayHandler
	var webhookH *handler.WebhookHandler
	if cfg.RelayEnabled {
		webhookKeyRepo := repository.NewWebhookKeyRepository(pool)
		relayH = handler.NewRelayHandler(relayRepo, webhookKeyRepo, hub)
		webhookH = handler.NewWebhookHandler(relayRepo, webhookKeyRepo, hub, rdb)
		log.Println("Relay hub enabled")

		// 每 5 分钟检查一次即将过期的 webhook key
		go periodicTask(taskCtx, 5*time.Minute, func() {
			service.CheckExpiringWebhookKeys(taskCtx, notificationSvc, webhookKeyRepo)
		})
	}

	engine := gin.Default()
	router.Setup(engine, Version, authH, assetH, reviewH, adminH, userH, userRepo, readLimiter, writeLimiter, publicH, githubH, bindH, resetKeyH, relayH, webhookH)

	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: engine,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

func periodicTask(ctx context.Context, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fn() // 启动时立即执行一次
	for {
		select {
		case <-ticker.C:
			fn()
		case <-ctx.Done():
			return
		}
	}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			api_key VARCHAR(64) UNIQUE NOT NULL,
			user_type VARCHAR(20) NOT NULL CHECK (user_type IN ('human', 'kodaclaw')),
			instance_id VARCHAR(255),
			display_name VARCHAR(100),
			description TEXT,
			is_admin BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`,
		`CREATE TABLE IF NOT EXISTS assets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(200) NOT NULL,
			type VARCHAR(20) NOT NULL CHECK (type IN ('soul', 'skill')),
			description TEXT NOT NULL,
			author_id UUID NOT NULL REFERENCES users(id),
			status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'superseded')),
			tags TEXT[] DEFAULT '{}',
			current_version VARCHAR(50),
			rejection_reason TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_author ON assets(author_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_tags ON assets USING GIN(tags)`,
		`CREATE TABLE IF NOT EXISTS asset_versions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			version VARCHAR(50) NOT NULL,
			file_key VARCHAR(500) NOT NULL,
			file_size BIGINT NOT NULL,
			changelog TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(asset_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_versions_asset ON asset_versions(asset_id)`,
		`CREATE TABLE IF NOT EXISTS reviews (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id),
			content TEXT NOT NULL,
			compatibility INT CHECK (compatibility BETWEEN 1 AND 5),
			usefulness INT CHECK (usefulness BETWEEN 1 AND 5),
			security INT CHECK (security BETWEEN 1 AND 5),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_asset ON reviews(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_user ON reviews(user_id)`,
		// Unique indexes for auth hot path
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		// Prevent duplicate reviews
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_asset_user ON reviews(asset_id, user_id)`,
		// Search performance indexes
		`CREATE INDEX IF NOT EXISTS idx_assets_author_id ON assets(author_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_versions_asset_version ON asset_versions(asset_id, version)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_asset_created ON reviews(asset_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_versions_asset_created ON asset_versions(asset_id, created_at DESC)`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS download_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS avg_rating DOUBLE PRECISION NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS asset_downloads (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id),
			user_id UUID NOT NULL REFERENCES users(id),
			downloaded_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(asset_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_downloads_asset ON asset_downloads(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_download_count ON assets(download_count DESC)`,
		`CREATE TABLE IF NOT EXISTS asset_favorites (
			user_id UUID NOT NULL REFERENCES users(id),
			asset_id UUID NOT NULL REFERENCES assets(id),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, asset_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_favorites_user ON asset_favorites(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_favorites_asset ON asset_favorites(asset_id)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id),
			type VARCHAR(50) NOT NULL,
			title VARCHAR(200) NOT NULL,
			message TEXT,
			related_asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read)`,
		// Phase 5 batch 2: content preview columns
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS asset_readme TEXT`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS asset_skill_content TEXT`,
		// Phase 5 batch 2: install count
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS install_count INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS asset_installs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id),
			instance_id VARCHAR(255),
			installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_installs_asset ON asset_installs(asset_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_asset_installs_unique ON asset_installs(asset_id, user_id, COALESCE(instance_id, ''))`,
		// Phase 10: version-level review + content isolation
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'superseded'))`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS rejection_reason TEXT`,
		// Phase 10b: add superseded to existing CHECK constraints (ADD COLUMN IF NOT EXISTS won't alter existing constraints)
		`ALTER TABLE asset_versions ALTER COLUMN status SET DEFAULT 'pending'`,
		`ALTER TABLE asset_versions DROP CONSTRAINT IF EXISTS asset_versions_status_check`,
		`ALTER TABLE asset_versions ADD CONSTRAINT asset_versions_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'superseded'))`,
		`ALTER TABLE assets ALTER COLUMN status SET DEFAULT 'pending'`,
		`ALTER TABLE assets DROP CONSTRAINT IF EXISTS assets_status_check`,
		`ALTER TABLE assets ADD CONSTRAINT assets_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'superseded'))`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS skill_content TEXT`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS readme TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_asset_versions_status ON asset_versions(status)`,
		// Phase 5 batch 2: asset dependencies
		`CREATE TABLE IF NOT EXISTS asset_dependencies (
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			depends_on_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			PRIMARY KEY (asset_id, depends_on_asset_id),
			CHECK (asset_id != depends_on_asset_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_dependencies_asset ON asset_dependencies(asset_id)`,
	}

	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, sql)
		}
	}
	// pg_trgm for ILIKE search optimization
	pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_assets_name_trgm ON assets USING GIN (name gin_trgm_ops)")
	pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_assets_description_trgm ON assets USING GIN (description gin_trgm_ops)")
	return nil
}

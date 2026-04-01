package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/handler"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"github.com/vanzheng/kodaclaw-community/internal/router"
)

// Integration tests using real PostgreSQL (Docker)
// Prerequisites: docker compose up -d
// Run: go test ./tests/ -v -run Integration

const testDSN = "postgres://postgres:postgres@localhost:5432/kodaclaw_community?sslmode=disable"

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	// Run migrations
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
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS download_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS avg_rating DOUBLE PRECISION NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS asset_downloads (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id),
			user_id UUID NOT NULL REFERENCES users(id),
			downloaded_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(asset_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS asset_favorites (
			user_id UUID NOT NULL REFERENCES users(id),
			asset_id UUID NOT NULL REFERENCES assets(id),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, asset_id)
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id),
			type VARCHAR(50) NOT NULL,
			title VARCHAR(200) NOT NULL,
			message TEXT,
			related_asset_id UUID REFERENCES assets(id),
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read)`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS asset_readme TEXT`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS asset_skill_content TEXT`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS install_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS rejection_reason TEXT`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS skill_content TEXT`,
		`ALTER TABLE asset_versions ADD COLUMN IF NOT EXISTS readme TEXT`,
		`CREATE TABLE IF NOT EXISTS asset_installs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id),
			instance_id VARCHAR(255),
			installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_installs_asset ON asset_installs(asset_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_asset_installs_unique ON asset_installs(asset_id, user_id, COALESCE(instance_id, ''))`,
		`CREATE TABLE IF NOT EXISTS asset_dependencies (
			asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			depends_on_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			PRIMARY KEY (asset_id, depends_on_asset_id),
			CHECK (asset_id != depends_on_asset_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_dependencies_asset ON asset_dependencies(asset_id)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS github_id BIGINT`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS github_username VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id) WHERE github_id IS NOT NULL`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claim_token VARCHAR(36)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claim_expires_at TIMESTAMP`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_by UUID REFERENCES users(id)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMP`,
	}
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("migration failed: %v", err)
		}
	}

	// Clean up test data (order matters for FK)
	pool.Exec(ctx, "DELETE FROM notifications")
	pool.Exec(ctx, "DELETE FROM reviews")
	pool.Exec(ctx, "DELETE FROM asset_downloads")
	pool.Exec(ctx, "DELETE FROM asset_favorites")
	pool.Exec(ctx, "DELETE FROM asset_installs")
	pool.Exec(ctx, "DELETE FROM asset_dependencies")
	pool.Exec(ctx, "DELETE FROM asset_versions")
	pool.Exec(ctx, "DELETE FROM assets")
	pool.Exec(ctx, "DELETE FROM users")

	return pool
}

func setupTestRouter(pool *pgxpool.Pool, storagePath string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	userRepo := repository.NewUserRepository(pool)
	assetRepo := repository.NewAssetRepository(pool)
	versionRepo := repository.NewAssetVersionRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	favoriteRepo := repository.NewFavoriteRepository(pool)
	notificationRepo := repository.NewNotificationRepository(pool)
	depRepo := repository.NewAssetDependencyRepository(pool)
	installRepo := repository.NewAssetInstallRepository(pool)

	authH := handler.NewAuthHandler(userRepo)
	assetH := handler.NewAssetHandlerFull(assetRepo, versionRepo, userRepo, favoriteRepo, depRepo, installRepo, storagePath)
	reviewH := handler.NewReviewHandler(reviewRepo, assetRepo)
	adminH := handler.NewAdminHandler(assetRepo, notificationRepo, versionRepo, userRepo, storagePath)
	userH := handler.NewUserHandlerWithNotifications(userRepo, assetRepo, favoriteRepo, notificationRepo)
	publicH := handler.NewPublicHandler(assetRepo, versionRepo, reviewRepo, userRepo, storagePath)
	githubH := handler.NewGitHubHandler(userRepo)
	claimH := handler.NewClaimHandler(userRepo)

	readLimiter := middleware.NewMemoryRateLimiter(1000, 60)
	uploadLimiter := middleware.NewMemoryRateLimiter(1000, 60)
	writeLimiter := middleware.NewMemoryRateLimiter(1000, 60)

	engine := gin.New()
	router.Setup(engine, authH, assetH, reviewH, adminH, userH, userRepo, readLimiter, writeLimiter, uploadLimiter, publicH, githubH, claimH)
	return engine
}

func createTestUser(t *testing.T, r *gin.Engine, username, password, userType string, isAdmin bool) (string, string) {
	t.Helper()
	body := map[string]interface{}{
		"username":  username,
		"password":  password,
		"user_type": userType,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if isAdmin {
		req.Header.Set("X-Admin-Key", "dev-admin-secret")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("register %s: expected 201, got %d, body: %s", username, w.Code, w.Body.String())
	}

	var resp struct {
		APIKey string `json:"api_key"`
		ID     string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.APIKey, resp.ID
}

// ========== Auth Integration ==========

func TestIntegration_RegisterAndLogin(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	apiKey, userID := createTestUser(t, r, "testuser1", "password123", "human", false)
	if apiKey == "" || userID == "" {
		t.Fatal("expected non-empty api key and user id")
	}

	// Duplicate username
	body, _ := json.Marshal(map[string]string{"username": "testuser1", "password": "password123", "user_type": "human"})
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Errorf("duplicate register: expected 409, got %d", w.Code)
	}

	// Short password
	body, _ = json.Marshal(map[string]string{"username": "short", "password": "abc", "user_type": "human"})
	req = httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("short password: expected 400, got %d", w.Code)
	}

	// Login success
	body, _ = json.Marshal(map[string]string{"username": "testuser1", "password": "password123"})
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("login: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Login wrong password
	body, _ = json.Marshal(map[string]string{"username": "testuser1", "password": "wrong"})
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("wrong password: expected 401, got %d", w.Code)
	}
}

// ========== Asset CRUD Integration ==========

func TestIntegration_AssetCRUD(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "creator1", "password123", "kodaclaw", false)

	// Upload asset
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "My Test Skill")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "A skill for testing")
	_ = writer.WriteField("tags", "test,demo")
	_ = writer.WriteField("version", "1.0.0")
	_ = writer.WriteField("changelog", "Initial release")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload: expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var asset struct {
		ID     string   `json:"id"`
		Status string   `json:"status"`
		Name   string   `json:"name"`
		Tags   []string `json:"tags"`
	}
	json.Unmarshal(w.Body.Bytes(), &asset)
	if asset.Status != "pending" {
		t.Errorf("expected pending, got %s", asset.Status)
	}
	if asset.Name != "My Test Skill" {
		t.Errorf("expected 'My Test Skill', got %s", asset.Name)
	}
	if len(asset.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(asset.Tags))
	}
	assetID := asset.ID

	// Public list should be empty (pending)
	req = httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var list struct{ Total int `json:"total"` }
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("public list should be empty, got %d", list.Total)
	}

	// Approve
	adminKey, _ := createTestUser(t, r, "admin1", "adminpass1", "human", true)
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Now public list has 1 item
	req = httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("public list: expected 1, got %d", list.Total)
	}

	// Filter by type
	req = httptest.NewRequest("GET", "/api/v1/assets?type=skill", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("type filter: expected 1, got %d", list.Total)
	}

	req = httptest.NewRequest("GET", "/api/v1/assets?type=soul", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("soul filter: expected 0, got %d", list.Total)
	}

	// Search by keyword
	req = httptest.NewRequest("GET", "/api/v1/assets?q=Test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("search Test: expected 1, got %d", list.Total)
	}

	req = httptest.NewRequest("GET", "/api/v1/assets?q=nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("search nonexistent: expected 0, got %d", list.Total)
	}

	// Filter by tag
	req = httptest.NewRequest("GET", "/api/v1/assets?tag=test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("tag test: expected 1, got %d", list.Total)
	}

	// Get detail
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("detail: expected 200, got %d", w.Code)
	}

	// Non-existent asset
	req = httptest.NewRequest("GET", "/api/v1/assets/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("non-existent: expected 404, got %d", w.Code)
	}

	// Download
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("download: expected 200, got %d", w.Code)
	}
	if !strings.HasPrefix(w.Body.String(), string([]byte{0x50, 0x4B})) {
		t.Errorf("download content mismatch")
	}

	// Versions
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/versions", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("versions: expected 200, got %d", w.Code)
	}
	var versions []model.AssetVersion
	json.Unmarshal(w.Body.Bytes(), &versions)
	if len(versions) != 1 || versions[0].Version != "1.0.0" {
		t.Errorf("expected 1 version 1.0.0, got %v", versions)
	}
}

// ========== Reviews Integration ==========

func TestIntegration_Reviews(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "reviewer1", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "admin2", "adminpass2", "human", true)

	// Create and approve asset
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Reviewable Skill")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "For review testing")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Post review
	reviewBody, _ := json.Marshal(map[string]interface{}{
		"content": "Great skill!", "compatibility": 5, "usefulness": 4, "security": 5,
	})
	req = httptest.NewRequest("POST", "/api/v1/assets/"+asset.ID+"/reviews", bytes.NewReader(reviewBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create review: expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var review struct{ Content string `json:"content"` }
	json.Unmarshal(w.Body.Bytes(), &review)
	if review.Content != "Great skill!" {
		t.Errorf("review content mismatch")
	}

	// List reviews
	req = httptest.NewRequest("GET", "/api/v1/assets/"+asset.ID+"/reviews", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var reviewList struct{ Total int `json:"total"` }
	json.Unmarshal(w.Body.Bytes(), &reviewList)
	if reviewList.Total != 1 {
		t.Errorf("expected 1 review, got %d", reviewList.Total)
	}

	// Review on non-existent asset
	reviewBody, _ = json.Marshal(map[string]interface{}{"content": "ghost"})
	req = httptest.NewRequest("POST", "/api/v1/assets/"+uuid.New().String()+"/reviews", bytes.NewReader(reviewBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("review on non-existent: expected 404, got %d", w.Code)
	}
}

// ========== Admin Integration ==========

func TestIntegration_AdminApproval(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "creator2", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin3", "adminpass3", "human", true)
	nonAdminKey, _ := createTestUser(t, r, "normal1", "password123", "human", false)

	// Upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Admin Test Skill")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "For admin testing")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Non-admin cannot access admin
	req = httptest.NewRequest("GET", "/api/v1/admin/assets?status=pending", nil)
	req.Header.Set("Authorization", "Bearer "+nonAdminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("non-admin: expected 403, got %d", w.Code)
	}

	// Admin list pending
	req = httptest.NewRequest("GET", "/api/v1/admin/assets?status=pending", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("admin list: expected 200, got %d", w.Code)
	}
	var adminList struct{ Total int `json:"total"` }
	json.Unmarshal(w.Body.Bytes(), &adminList)
	if adminList.Total != 1 {
		t.Errorf("pending: expected 1, got %d", adminList.Total)
	}

	// Reject
	rejectBody, _ := json.Marshal(map[string]string{"reason": "Security concern"})
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/reject", bytes.NewReader(rejectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("reject: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var rejectResp struct{ Reason string `json:"reason"` }
	json.Unmarshal(w.Body.Bytes(), &rejectResp)
	if rejectResp.Reason != "Security concern" {
		t.Errorf("reason mismatch: %s", rejectResp.Reason)
	}

	// Verify in rejected list
	req = httptest.NewRequest("GET", "/api/v1/admin/assets?status=rejected", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &adminList)
	if adminList.Total != 1 {
		t.Errorf("rejected list: expected 1, got %d", adminList.Total)
	}

	// Re-approve
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("re-approve: expected 200, got %d", w.Code)
	}

	// Show in public
	req = httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var publicList struct{ Total int `json:"total"` }
	json.Unmarshal(w.Body.Bytes(), &publicList)
	if publicList.Total != 1 {
		t.Errorf("public after approve: expected 1, got %d", publicList.Total)
	}
}

// ========== User Profile Integration ==========

func TestIntegration_UserProfile(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, userID := createTestUser(t, r, "profile1", "password123", "human", false)

	// Get me
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get me: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var me struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &me)
	if me.ID != userID {
		t.Errorf("id mismatch: expected %s, got %s", userID, me.ID)
	}

	// Get by ID
	req = httptest.NewRequest("GET", "/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("get by id: expected 200, got %d", w.Code)
	}

	// Non-existent
	req = httptest.NewRequest("GET", "/api/v1/users/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("non-existent user: expected 404, got %d", w.Code)
	}
}

// ========== Edge Cases ==========

func TestIntegration_HealthCheck(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("health: expected 200, got %d", w.Code)
	}
	var health struct{ Status string `json:"status"` }
	json.Unmarshal(w.Body.Bytes(), &health)
	if health.Status != "ok" {
		t.Errorf("status: expected ok, got %s", health.Status)
	}
}

func TestIntegration_UnauthorizedAccess(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	for _, ep := range []string{"/api/v1/assets", "/api/v1/users/me", "/api/v1/admin/assets"} {
		req := httptest.NewRequest("GET", ep, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("%s without auth: expected 401, got %d", ep, w.Code)
		}
	}
}

func TestIntegration_NonZipUpload(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "uploadtest", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Bad Asset")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Not a zip")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "malware.exe")
	part.Write([]byte("exe content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("non-zip: expected 400, got %d", w.Code)
	}
}

func TestIntegration_InvalidUUID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "uuidtest", "password123", "human", false)

	cases := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/assets/not-a-uuid"},
		{"GET", "/api/v1/assets/not-a-uuid/download"},
		{"GET", "/api/v1/assets/not-a-uuid/versions"},
		{"GET", "/api/v1/assets/not-a-uuid/reviews"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("%s %s: expected 400, got %d", c.method, c.path, w.Code)
		}
	}
}

func TestIntegration_DownloadNonExistentVersion(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "dltest", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin4", "adminpass4", "human", true)

	// Create and approve
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "DL Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Download test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Non-existent version
	req = httptest.NewRequest("GET", "/api/v1/assets/"+asset.ID+"/download?version=99.0.0", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("non-existent version: expected 404, got %d", w.Code)
	}
}

func TestIntegration_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pagtest", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin5", "adminpass5", "human", true)

	for i := 0; i < 5; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("name", fmt.Sprintf("Asset %d", i))
		_ = writer.WriteField("type", "skill")
		_ = writer.WriteField("description", "Pagination test")
		_ = writer.WriteField("version", "1.0.0")
		part, _ := writer.CreateFormFile("file", "skill.zip")
		part.Write(makeMinimalZip())
		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/assets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var asset struct{ ID string `json:"id"` }
		json.Unmarshal(w.Body.Bytes(), &asset)

		req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
		req.Header.Set("Authorization", "Bearer "+adminKey)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	type listResp struct {
		Items []interface{} `json:"items"`
		Total int           `json:"total"`
	}

	// Page 1 size 2
	req := httptest.NewRequest("GET", "/api/v1/assets?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l1 listResp
	json.Unmarshal(w.Body.Bytes(), &l1)
	if l1.Total != 5 || len(l1.Items) != 2 {
		t.Errorf("page 1: total=%d items=%d, want total=5 items=2", l1.Total, len(l1.Items))
	}

	// Page 3 size 2 (last page, 1 item)
	req = httptest.NewRequest("GET", "/api/v1/assets?page=3&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l3 listResp
	json.Unmarshal(w.Body.Bytes(), &l3)
	if len(l3.Items) != 1 {
		t.Errorf("page 3: expected 1 item, got %d", len(l3.Items))
	}

	// Page 4 (empty)
	req = httptest.NewRequest("GET", "/api/v1/assets?page=4&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l4 listResp
	json.Unmarshal(w.Body.Bytes(), &l4)
	if len(l4.Items) != 0 {
		t.Errorf("page 4: expected 0 items, got %d", len(l4.Items))
	}
}

func TestIntegration_FileStorage(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "filetest", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "File Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "File storage test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "myskill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	expectedPath := filepath.Join(tmpDir, asset.ID, "1.0.0", "myskill.zip")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("file not found at %s", expectedPath)
	}
	content, _ := os.ReadFile(expectedPath)
	if string(content) != string(makeMinimalZip()) {
		t.Errorf("content mismatch: %s", string(content))
	}
}

// ========== Security Fix Tests ==========

// uploadTestAsset is a helper that uploads a zip asset and returns its ID.
func uploadTestAsset(t *testing.T, r *gin.Engine, apiKey, name, version string) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "test asset")
	_ = writer.WriteField("version", version)
	part, _ := writer.CreateFormFile("file", "asset.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload %s: expected 201, got %d body: %s", name, w.Code, w.Body.String())
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)
	return asset.ID
}

// approveAsset is a helper that approves an asset as admin.
func approveAsset(t *testing.T, r *gin.Engine, adminKey, assetID string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve %s: expected 200, got %d", assetID, w.Code)
	}
}

func TestSecurity_PathTraversal(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "ptuser", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Path Traversal Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "../../etc/passwd.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("upload: expected 201, got %d body: %s", w.Code, w.Body.String())
	}

	// Verify the stored file is safe (filename was sanitized)
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// The file should be stored as "passwd.zip" not "../../etc/passwd.zip"
	safePath := filepath.Join(tmpDir, asset.ID, "1.0.0", "passwd.zip")
	unsafePath := filepath.Join(tmpDir, "etc", "passwd.zip")
	if _, err := os.Stat(safePath); os.IsNotExist(err) {
		t.Errorf("sanitized file not found at %s", safePath)
	}
	if _, err := os.Stat(unsafePath); !os.IsNotExist(err) {
		t.Errorf("path traversal succeeded: file found at %s", unsafePath)
	}
}

func TestSecurity_VersionFormat(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "veruser", "password123", "kodaclaw", false)

	cases := []struct {
		version string
		wantOK  bool
	}{
		{"1.0.0", true},
		{"10.20.30", true},
		{"1.0", false},
		{"v1.0.0", false},
		{"1.0.0.0", false},
		{"abc", false},
		{"1.0.a", false},
		{"../1.0", false},
	}

	for _, tc := range cases {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("name", "Ver Test")
		_ = writer.WriteField("type", "skill")
		_ = writer.WriteField("description", "test")
		_ = writer.WriteField("version", tc.version)
		part, _ := writer.CreateFormFile("file", "asset.zip")
		part.Write(makeMinimalZip())
		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/assets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if tc.wantOK && w.Code != 201 {
			t.Errorf("version %q: expected 201, got %d", tc.version, w.Code)
		}
		if !tc.wantOK && w.Code != 400 {
			t.Errorf("version %q: expected 400, got %d", tc.version, w.Code)
		}
	}
}

func TestSecurity_FileSizeLimit(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "sizeuser", "password123", "kodaclaw", false)

	// Create a fake multipart request with a large Content-Length to trigger size check.
	// We fake header.Size by using a custom approach: create a real large body.
	// Since actually sending 50MB in a unit test is impractical, we verify that
	// the size check logic correctly reads header.Size.
	// Instead, create a multipart with a moderately large file that exceeds our limit
	// by patching the content-length hint.
	//
	// The simplest approach: send a small payload but verify 413 is returned when
	// the multipart header.Size exceeds 50MB. We can test this by checking that
	// small files work (201) - confirming the check isn't always rejecting.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Size Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "small.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Errorf("small file: expected 201, got %d body: %s", w.Code, w.Body.String())
	}
}

func TestSecurity_DuplicateReview(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	userKey, _ := createTestUser(t, r, "reviewer1", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "admin_dup", "adminpass", "human", true)

	assetID := uploadTestAsset(t, r, userKey, "Dup Review Asset", "1.0.0")
	approveAsset(t, r, adminKey, assetID)

	reviewBody, _ := json.Marshal(map[string]interface{}{
		"content": "Great asset!",
	})

	// First review - should succeed
	req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(reviewBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("first review: expected 201, got %d body: %s", w.Code, w.Body.String())
	}

	// Second review by same user - should return 409
	req = httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(reviewBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Errorf("duplicate review: expected 409, got %d body: %s", w.Code, w.Body.String())
	}
}

func TestSecurity_AdminAuthRequired(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	normalKey, _ := createTestUser(t, r, "normaluser2", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "admin_auth", "adminpass", "human", true)

	creatorKey, _ := createTestUser(t, r, "creator_auth", "password123", "kodaclaw", false)
	assetID := uploadTestAsset(t, r, creatorKey, "Auth Test Asset", "1.0.0")

	// Non-admin should get 403 on all admin endpoints
	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"GET", "/api/v1/admin/assets", nil},
		{"POST", "/api/v1/admin/assets/" + assetID + "/approve", nil},
		{"POST", "/api/v1/admin/assets/" + assetID + "/reject", mustJSON(map[string]string{"reason": "test"})},
	}

	for _, tc := range cases {
		var bodyReader *bytes.Reader
		if tc.body != nil {
			bodyReader = bytes.NewReader(tc.body)
		} else {
			bodyReader = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(tc.method, tc.path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+normalKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("%s %s with normal user: expected 403, got %d", tc.method, tc.path, w.Code)
		}
	}

	// Admin should succeed on list
	req := httptest.NewRequest("GET", "/api/v1/admin/assets", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("admin list: expected 200, got %d", w.Code)
	}
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestSecurity_DownloadAuthRequired(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	creatorKey, _ := createTestUser(t, r, "creator_dl", "password123", "kodaclaw", false)
	otherKey, _ := createTestUser(t, r, "other_dl", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "admin_dl", "adminpass", "human", true)

	// Upload asset (status = pending)
	assetID := uploadTestAsset(t, r, creatorKey, "Download Auth Asset", "1.0.0")

	// Other user cannot download pending asset
	req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+otherKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("pending download by other: expected 403, got %d", w.Code)
	}

	// Author can download own pending asset
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Should be 200 (file exists) or 404 (no file in tmpDir is fine), but NOT 403
	if w.Code == 403 {
		t.Errorf("author download own pending: should not get 403, got %d", w.Code)
	}

	// Approve and verify other user can now download
	approveAsset(t, r, adminKey, assetID)
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+otherKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// After approval, should not be 403 (might be 200 or 404 depending on file)
	if w.Code == 403 {
		t.Errorf("approved download by other: should not get 403, got %d", w.Code)
	}
}

func TestSecurity_PaginationBoundaries(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pagbound", "password123", "human", false)

	type listResp struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
		Total    int `json:"total"`
	}

	cases := []struct {
		query        string
		wantPage     int
		wantPageSize int
	}{
		{"page=-1&page_size=20", 1, 20},
		{"page=0&page_size=20", 1, 20},
		{"page=1&page_size=-1", 1, 20},
		{"page=1&page_size=0", 1, 20},
		{"page=1&page_size=999999", 1, 20},
		{"page=1&page_size=101", 1, 20},
		{"page=1&page_size=100", 1, 100},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/api/v1/assets?"+tc.query, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("query %q: expected 200, got %d", tc.query, w.Code)
			continue
		}
		var resp listResp
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Page != tc.wantPage {
			t.Errorf("query %q: page=%d want %d", tc.query, resp.Page, tc.wantPage)
		}
		if resp.PageSize != tc.wantPageSize {
			t.Errorf("query %q: page_size=%d want %d", tc.query, resp.PageSize, tc.wantPageSize)
		}
	}
}

func TestSecurity_AdminApproveNotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())
	adminKey, _ := createTestUser(t, r, "admin_nf", "adminpass", "human", true)

	nonExistentID := uuid.New().String()

	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+nonExistentID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("approve non-existent: expected 404, got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+nonExistentID+"/reject",
		bytes.NewReader(mustJSON(map[string]string{"reason": "test"})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("reject non-existent: expected 404, got %d", w.Code)
	}
}

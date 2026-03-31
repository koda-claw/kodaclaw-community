package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

// createAndApproveAsset uploads a skill asset for apiKey and approves it with adminKey.
// Returns the asset ID string.
func createAndApproveAsset(t *testing.T, r *gin.Engine, apiKey, adminKey, name string) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Test asset: "+name)
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "asset.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload %s: expected 201, got %d, body: %s", name, w.Code, w.Body.String())
	}

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve %s: expected 200, got %d, body: %s", name, w.Code, w.Body.String())
	}

	return asset.ID
}

// ========== User Asset List Tests ==========

func TestIntegration_UserAssets_HasAssets(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, creatorID := createTestUser(t, r, "asset_author1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin_ua1", "adminpass1", "human", true)
	viewerKey, _ := createTestUser(t, r, "viewer_ua1", "password123", "human", false)

	createAndApproveAsset(t, r, creatorKey, adminKey, "Author Asset One")
	createAndApproveAsset(t, r, creatorKey, adminKey, "Author Asset Two")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+creatorID+"/assets", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("list user assets: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
}

func TestIntegration_UserAssets_NoAssets(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	_, userID := createTestUser(t, r, "no_asset_user1", "password123", "human", false)
	viewerKey, _ := createTestUser(t, r, "viewer_ua2", "password123", "human", false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID+"/assets", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("list user assets (none): expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
	if len(resp.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(resp.Items))
	}
}

func TestIntegration_UserAssets_UserNotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	viewerKey, _ := createTestUser(t, r, "viewer_ua3", "password123", "human", false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+uuid.New().String()+"/assets", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("unknown user assets: expected 404, got %d", w.Code)
	}
}

// ========== Search Author Filter Tests ==========

func TestIntegration_SearchAuthorFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	author1Key, author1ID := createTestUser(t, r, "search_author1", "password123", "kodaclaw", false)
	author2Key, _ := createTestUser(t, r, "search_author2", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin_sf1", "adminpass1", "human", true)
	viewerKey, _ := createTestUser(t, r, "viewer_sf1", "password123", "human", false)

	createAndApproveAsset(t, r, author1Key, adminKey, "Author1 Skill A")
	createAndApproveAsset(t, r, author1Key, adminKey, "Author1 Skill B")
	createAndApproveAsset(t, r, author2Key, adminKey, "Author2 Skill C")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets?author="+author1ID, nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("search by author: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("author filter: expected total=2, got %d", resp.Total)
	}
	for _, a := range resp.Items {
		if a.AuthorID.String() != author1ID {
			t.Errorf("author filter: unexpected author %s", a.AuthorID)
		}
	}
}

func TestIntegration_SearchAuthorFilter_InvalidUUID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "search_author3", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin_sf2", "adminpass1", "human", true)
	viewerKey, _ := createTestUser(t, r, "viewer_sf2", "password123", "human", false)

	createAndApproveAsset(t, r, authorKey, adminKey, "Some Skill X")

	// Invalid UUID — filter is ignored, all approved assets are returned
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets?author=not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("invalid author uuid: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total < 1 {
		t.Errorf("invalid author uuid: expected at least 1 asset, got %d", resp.Total)
	}
}

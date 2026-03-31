package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// toggleFavorite sends POST /api/v1/assets/:id/favorite and returns the status code and favorited value.
func toggleFavorite(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey, assetID string) (int, bool) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/favorite", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return w.Code, false
	}
	var result struct {
		Favorited bool `json:"favorited"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result.Favorited
}

// listFavorites sends GET /api/v1/users/me/favorites and returns total and items.
func listFavorites(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey string) (int, []map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/users/me/favorites", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list favorites: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var result struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result.Total, result.Items
}

func TestIntegration_Favorites_ToggleAddsAndRemoves(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "fav_creator1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "fav_admin1", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "fav_user1", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Fav Toggle Asset")

	// First toggle: should favorite
	code, favorited := toggleFavorite(t, r, userKey, assetID)
	if code != http.StatusOK {
		t.Fatalf("toggle favorite: expected 200, got %d", code)
	}
	if !favorited {
		t.Error("expected favorited=true after first toggle")
	}

	// Second toggle: should unfavorite
	code, favorited = toggleFavorite(t, r, userKey, assetID)
	if code != http.StatusOK {
		t.Fatalf("toggle unfavorite: expected 200, got %d", code)
	}
	if favorited {
		t.Error("expected favorited=false after second toggle")
	}
}

func TestIntegration_Favorites_ListEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	_, _ = createTestUser(t, r, "fav_creator2", "password123", "kodaclaw", false)
	userKey, _ := createTestUser(t, r, "fav_user2", "password123", "human", false)

	total, items := listFavorites(t, r, userKey)
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestIntegration_Favorites_ListNonEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "fav_creator3", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "fav_admin3", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "fav_user3", "password123", "human", false)

	assetID1 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Fav List Asset One")
	assetID2 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Fav List Asset Two")

	toggleFavorite(t, r, userKey, assetID1)
	toggleFavorite(t, r, userKey, assetID2)

	total, items := listFavorites(t, r, userKey)
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// Check asset_name and asset_type are present
	for _, item := range items {
		if item["asset_name"] == "" || item["asset_name"] == nil {
			t.Error("expected asset_name to be set")
		}
		if item["asset_type"] == "" || item["asset_type"] == nil {
			t.Error("expected asset_type to be set")
		}
	}
	_ = assetID1
	_ = assetID2
}

func TestIntegration_Favorites_NonExistentAsset(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	_, _ = createTestUser(t, r, "fav_creator4", "password123", "kodaclaw", false)
	userKey, _ := createTestUser(t, r, "fav_user4", "password123", "human", false)

	fakeID := "00000000-0000-0000-0000-000000000000"
	code, _ := toggleFavorite(t, r, userKey, fakeID)
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent asset, got %d", code)
	}
}

func TestIntegration_Favorites_DoubleFavoriteThenUnfav(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "fav_creator5", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "fav_admin5", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "fav_user5", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Fav Double Toggle Asset")

	// Toggle once → favorited
	_, fav1 := toggleFavorite(t, r, userKey, assetID)
	if !fav1 {
		t.Error("expected favorited=true after 1st toggle")
	}

	// Toggle again → unfavorited
	_, fav2 := toggleFavorite(t, r, userKey, assetID)
	if fav2 {
		t.Error("expected favorited=false after 2nd toggle")
	}

	// Toggle again → favorited again
	_, fav3 := toggleFavorite(t, r, userKey, assetID)
	if !fav3 {
		t.Error("expected favorited=true after 3rd toggle")
	}

	// Verify in list
	total, _ := listFavorites(t, r, userKey)
	if total != 1 {
		t.Errorf("expected total=1 in favorites list, got %d", total)
	}
}

func TestIntegration_Favorites_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "fav_creator6", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "fav_admin6", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "fav_user6", "password123", "human", false)

	// Create 3 assets and favorite all
	for i := 0; i < 3; i++ {
		assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Fav Pagination Asset")
		toggleFavorite(t, r, userKey, assetID)
	}

	// Page 1 with page_size=2
	req := httptest.NewRequest("GET", "/api/v1/users/me/favorites?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pagination page 1: expected 200, got %d", w.Code)
	}

	var page1 struct {
		Items    []map[string]interface{} `json:"items"`
		Total    int                      `json:"total"`
		Page     int                      `json:"page"`
		PageSize int                      `json:"page_size"`
	}
	json.Unmarshal(w.Body.Bytes(), &page1)

	if page1.Total != 3 {
		t.Errorf("expected total=3, got %d", page1.Total)
	}
	if len(page1.Items) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(page1.Items))
	}
	if page1.Page != 1 {
		t.Errorf("expected page=1, got %d", page1.Page)
	}

	// Page 2 with page_size=2
	req = httptest.NewRequest("GET", "/api/v1/users/me/favorites?page=2&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pagination page 2: expected 200, got %d", w.Code)
	}

	var page2 struct {
		Items []map[string]interface{} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &page2)

	if len(page2.Items) != 1 {
		t.Errorf("expected 1 item on page 2, got %d", len(page2.Items))
	}
}

func TestIntegration_Favorites_IsFavoritedInAsset(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "fav_creator7", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "fav_admin7", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "fav_user7", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "IsFavorited Test Asset")

	// Before favoriting: is_favorited should be false (omitted)
	req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get asset: expected 200, got %d", w.Code)
	}
	var asset struct {
		IsFavorited bool `json:"is_favorited"`
	}
	json.Unmarshal(w.Body.Bytes(), &asset)
	if asset.IsFavorited {
		t.Error("expected is_favorited=false before favoriting")
	}

	// Favorite the asset
	toggleFavorite(t, r, userKey, assetID)

	// After favoriting: is_favorited should be true
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &asset)
	if !asset.IsFavorited {
		t.Error("expected is_favorited=true after favoriting")
	}
}

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ========== Download Statistics Tests ==========

// downloadAsset sends GET /api/v1/assets/:id/download and returns the status code.
func downloadAsset(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey, assetID string) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// getAssetDownloadCount fetches an asset and returns its download_count.
func getAssetDownloadCount(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey, assetID string) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get asset: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var asset struct {
		DownloadCount int `json:"download_count"`
	}
	json.Unmarshal(w.Body.Bytes(), &asset)
	return asset.DownloadCount
}

func TestIntegration_Download_IncrementsCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin1", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "dl_user1", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Download Count Test Asset")

	// Initial count should be 0
	count := getAssetDownloadCount(t, r, userKey, assetID)
	if count != 0 {
		t.Errorf("initial download_count: expected 0, got %d", count)
	}

	// Download the asset
	code := downloadAsset(t, r, userKey, assetID)
	if code != 200 {
		t.Fatalf("download: expected 200, got %d", code)
	}

	// Wait briefly for goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// Count should now be 1
	count = getAssetDownloadCount(t, r, userKey, assetID)
	if count != 1 {
		t.Errorf("after download, download_count: expected 1, got %d", count)
	}
}

func TestIntegration_Download_DuplicateDoesNotDoubleCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator2", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin2", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "dl_user2", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Duplicate Download Test Asset")

	// Same user downloads the asset twice
	code := downloadAsset(t, r, userKey, assetID)
	if code != 200 {
		t.Fatalf("first download: expected 200, got %d", code)
	}
	code = downloadAsset(t, r, userKey, assetID)
	if code != 200 {
		t.Fatalf("second download: expected 200, got %d", code)
	}

	// Wait briefly for goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Count should still be 1 (duplicate user download not counted)
	count := getAssetDownloadCount(t, r, userKey, assetID)
	if count != 1 {
		t.Errorf("after duplicate download, download_count: expected 1, got %d", count)
	}
}

func TestIntegration_Download_MultipleUsersEachCountOnce(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator3", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin3", "password123", "human", true)
	userKey1, _ := createTestUser(t, r, "dl_user3a", "password123", "human", false)
	userKey2, _ := createTestUser(t, r, "dl_user3b", "password123", "human", false)
	userKey3, _ := createTestUser(t, r, "dl_user3c", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Multi-User Download Test")

	// Three different users download
	downloadAsset(t, r, userKey1, assetID)
	downloadAsset(t, r, userKey2, assetID)
	downloadAsset(t, r, userKey3, assetID)

	// Wait briefly for goroutines to complete
	time.Sleep(150 * time.Millisecond)

	count := getAssetDownloadCount(t, r, userKey1, assetID)
	if count != 3 {
		t.Errorf("three users downloaded, download_count: expected 3, got %d", count)
	}
}

func TestIntegration_SearchSort_ByDownloads(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator4", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin4", "password123", "human", true)
	userKey1, _ := createTestUser(t, r, "dl_user4a", "password123", "human", false)
	userKey2, _ := createTestUser(t, r, "dl_user4b", "password123", "human", false)

	// Create two assets
	assetID1 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Sort Test Asset Alpha")
	assetID2 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Sort Test Asset Beta")

	// assetID2 gets more downloads (2 users), assetID1 gets fewer (1 user)
	downloadAsset(t, r, userKey1, assetID2)
	downloadAsset(t, r, userKey2, assetID2)
	downloadAsset(t, r, userKey1, assetID1)

	// Wait for goroutines
	time.Sleep(150 * time.Millisecond)

	// Search sorted by downloads
	req := httptest.NewRequest("GET", "/api/v1/assets?sort=downloads", nil)
	req.Header.Set("Authorization", "Bearer "+userKey1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("search: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Items []struct {
			ID            string `json:"id"`
			DownloadCount int    `json:"download_count"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)

	if len(result.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(result.Items))
	}

	// First item should have more downloads than second
	if result.Items[0].DownloadCount < result.Items[1].DownloadCount {
		t.Errorf("expected sort by downloads DESC: first=%d, second=%d",
			result.Items[0].DownloadCount, result.Items[1].DownloadCount)
	}

	// The most downloaded asset should be assetID2
	if result.Items[0].ID != assetID2 {
		t.Errorf("expected first item to be assetID2 (%s), got %s", assetID2, result.Items[0].ID)
	}
}

func TestIntegration_SearchSort_DefaultCreatedAt(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator5", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin5", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "dl_user5", "password123", "human", false)

	// Create two assets — second one is newer
	assetID1 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Created Sort Asset First")
	assetID2 := uploadAndApproveAsset(t, r, creatorKey, adminKey, "Created Sort Asset Second")

	// Search without sort (defaults to created_at DESC — newest first)
	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("search: expected 200, got %d", w.Code)
	}

	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)

	if len(result.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(result.Items))
	}

	// Newest (assetID2) should come first
	if result.Items[0].ID != assetID2 {
		t.Errorf("expected newest asset (%s) first, got %s", assetID2, result.Items[0].ID)
	}
	_ = assetID1
}

func TestIntegration_DownloadCount_InListResponse(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "dl_creator6", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "dl_admin6", "password123", "human", true)
	userKey, _ := createTestUser(t, r, "dl_user6", "password123", "human", false)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "List Response Count Test")

	downloadAsset(t, r, userKey, assetID)
	time.Sleep(100 * time.Millisecond)

	// Verify download_count appears in list results
	req := httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+userKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	var result struct {
		Items []struct {
			ID            string `json:"id"`
			DownloadCount int    `json:"download_count"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)

	found := false
	for _, item := range result.Items {
		if item.ID == assetID {
			found = true
			if item.DownloadCount != 1 {
				t.Errorf("expected download_count=1 in list, got %d", item.DownloadCount)
			}
		}
	}
	if !found {
		t.Errorf("asset %s not found in list response", assetID)
	}
}

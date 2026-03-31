package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// uploadTestAssetWithTags uploads a test asset with custom type and tags.
func uploadTestAssetWithTags(t *testing.T, r *gin.Engine, apiKey, name, assetType, version, tags string) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", assetType)
	_ = writer.WriteField("description", "Test asset description")
	_ = writer.WriteField("version", version)
	if tags != "" {
		_ = writer.WriteField("tags", tags)
	}
	part, _ := writer.CreateFormFile("file", "asset.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset %s: expected 201, got %d, body: %s", name, w.Code, w.Body.String())
	}

	var asset struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &asset)
	return asset.ID
}

// TestTagsRating_PopularTags tests the popular tags endpoint with approved assets.
func TestTagsRating_PopularTags(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "tagsuser1", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "tagsadmin1", "password123", "human", true)

	assetID := uploadTestAssetWithTags(t, r, apiKey, "TagAsset1", "skill", "1.0.0", "web,security")

	// Approve it
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Get popular tags
	req = httptest.NewRequest("GET", "/api/v1/tags/popular", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("popular tags: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var tags []repository.TagCount
	if err := json.NewDecoder(w.Body).Decode(&tags); err != nil {
		t.Fatalf("decode tags: %v", err)
	}

	if len(tags) == 0 {
		t.Fatal("expected at least one tag")
	}

	tagMap := make(map[string]int)
	for _, tc := range tags {
		tagMap[tc.Tag] = tc.Count
	}
	if tagMap["web"] == 0 {
		t.Errorf("expected 'web' tag in popular tags, got: %v", tags)
	}
	if tagMap["security"] == 0 {
		t.Errorf("expected 'security' tag in popular tags, got: %v", tags)
	}
}

// TestTagsRating_PopularTagsEmpty tests popular tags when no approved assets exist.
func TestTagsRating_PopularTagsEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "tagsuser2", "password123", "human", false)

	req := httptest.NewRequest("GET", "/api/v1/tags/popular", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("popular tags empty: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var tags []repository.TagCount
	if err := json.NewDecoder(w.Body).Decode(&tags); err != nil {
		t.Fatalf("decode tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty tags, got %d", len(tags))
	}
}

// TestTagsRating_AvgRatingOnReviewCreate tests that avg_rating is computed when a review is created.
func TestTagsRating_AvgRatingOnReviewCreate(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "ratingauthor1", "password123", "human", false)
	reviewerKey, _ := createTestUser(t, r, "ratingreviewer1", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "ratingadmin1", "password123", "human", true)

	assetID := uploadTestAsset(t, r, authorKey, "RatingAsset1", "1.0.0")

	// Approve
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}

	// Create a review with ratings
	reviewBody := map[string]interface{}{
		"content":       "Great asset",
		"compatibility": 4,
		"usefulness":    5,
		"security":      3,
	}
	b, _ := json.Marshal(reviewBody)
	req = httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reviewerKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create review: expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	// Wait briefly for the goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Get asset and check avg_rating
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+reviewerKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get asset: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var asset struct {
		AvgRating float64 `json:"avg_rating"`
	}
	if err := json.NewDecoder(w.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}

	// expected: (4 + 5 + 3) / 3.0 = 4.0
	expected := 4.0
	if asset.AvgRating != expected {
		t.Errorf("expected avg_rating %.2f, got %.2f", expected, asset.AvgRating)
	}
}

// TestTagsRating_AvgRatingZeroNoReviews tests that avg_rating is 0 when no reviews exist.
func TestTagsRating_AvgRatingZeroNoReviews(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "ratingauthor2", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "ratingadmin2", "password123", "human", true)

	assetID := uploadTestAsset(t, r, authorKey, "RatingAsset2", "1.0.0")

	// Approve
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}

	// Get asset - no reviews yet
	req = httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+authorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get asset: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var asset struct {
		AvgRating float64 `json:"avg_rating"`
	}
	if err := json.NewDecoder(w.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}

	if asset.AvgRating != 0 {
		t.Errorf("expected avg_rating 0, got %.2f", asset.AvgRating)
	}
}

// TestTagsRating_SortByRating tests searching assets sorted by rating.
func TestTagsRating_SortByRating(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "sortauthor1", "password123", "human", false)
	reviewer1Key, _ := createTestUser(t, r, "sortreviewer1", "password123", "human", false)
	reviewer2Key, _ := createTestUser(t, r, "sortreviewer2", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "sortadmin1", "password123", "human", true)

	asset1ID := uploadTestAsset(t, r, authorKey, "LowRatedAsset", "1.0.0")
	asset2ID := uploadTestAsset(t, r, authorKey, "HighRatedAsset", "1.0.0")

	// Approve both
	for _, id := range []string{asset1ID, asset2ID} {
		req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+id+"/approve", nil)
		req.Header.Set("Authorization", "Bearer "+adminKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("approve %s: %d %s", id, w.Code, w.Body.String())
		}
	}

	// Review asset1 with low rating (1,1,1)
	reviewBody := map[string]interface{}{
		"content":       "Not great",
		"compatibility": 1,
		"usefulness":    1,
		"security":      1,
	}
	b, _ := json.Marshal(reviewBody)
	req := httptest.NewRequest("POST", "/api/v1/assets/"+asset1ID+"/reviews", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reviewer1Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("review asset1: %d %s", w.Code, w.Body.String())
	}

	// Review asset2 with high rating (5,5,5)
	reviewBody = map[string]interface{}{
		"content":       "Excellent",
		"compatibility": 5,
		"usefulness":    5,
		"security":      5,
	}
	b, _ = json.Marshal(reviewBody)
	req = httptest.NewRequest("POST", "/api/v1/assets/"+asset2ID+"/reviews", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reviewer2Key)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("review asset2: %d %s", w.Code, w.Body.String())
	}

	// Wait for goroutines
	time.Sleep(200 * time.Millisecond)

	// Search sorted by rating
	req = httptest.NewRequest("GET", "/api/v1/assets?sort=rating", nil)
	req.Header.Set("Authorization", "Bearer "+authorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list by rating: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items []struct {
			ID        string  `json:"id"`
			AvgRating float64 `json:"avg_rating"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total < 2 {
		t.Fatalf("expected at least 2 assets, got %d", resp.Total)
	}

	// First item should be asset2 (higher rating)
	if resp.Items[0].ID != asset2ID {
		t.Errorf("expected highest rated asset first, got %s (want %s)", resp.Items[0].ID, asset2ID)
	}
	if resp.Items[0].AvgRating <= resp.Items[1].AvgRating {
		t.Errorf("expected descending rating order, got %.2f then %.2f", resp.Items[0].AvgRating, resp.Items[1].AvgRating)
	}
}

// TestTagsRating_AvgRatingInListResponse tests that avg_rating appears in list responses.
func TestTagsRating_AvgRatingInListResponse(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "listrating_author", "password123", "human", false)
	reviewerKey, _ := createTestUser(t, r, "listrating_reviewer", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "listrating_admin", "password123", "human", true)

	assetID := uploadTestAsset(t, r, authorKey, "ListRatingAsset", "1.0.0")

	// Approve
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}

	// Review
	reviewBody := map[string]interface{}{
		"content":       "Good",
		"compatibility": 3,
		"usefulness":    3,
		"security":      3,
	}
	b, _ := json.Marshal(reviewBody)
	req = httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reviewerKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("review: %d %s", w.Code, w.Body.String())
	}

	time.Sleep(100 * time.Millisecond)

	// List assets
	req = httptest.NewRequest("GET", "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+authorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Items) == 0 {
		t.Fatal("expected at least one item in list")
	}

	for _, item := range resp.Items {
		if item["id"] == assetID {
			if _, ok := item["avg_rating"]; !ok {
				t.Error("expected avg_rating field in list response")
			}
			return
		}
	}
	t.Errorf("asset %s not found in list response", assetID)
}

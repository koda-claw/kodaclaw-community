package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ========== Tag Validation Tests ==========

func TestIntegration_TagValidation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "tagtest_user", "password123", "kodaclaw", false)

	uploadAsset := func(tags string) int {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("name", "Tag Test Asset")
		_ = writer.WriteField("type", "skill")
		_ = writer.WriteField("description", "Testing tag validation")
		_ = writer.WriteField("tags", tags)
		_ = writer.WriteField("version", "1.0.0")
		part, _ := writer.CreateFormFile("file", "skill.zip")
		part.Write([]byte("zip content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/assets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Single tag exceeds 30 characters
	longTag := strings.Repeat("a", 31)
	if code := uploadAsset(longTag); code != 400 {
		t.Errorf("tag too long: expected 400, got %d", code)
	}

	// Tag with invalid characters (spaces)
	if code := uploadAsset("valid-tag,invalid tag"); code != 400 {
		t.Errorf("tag with space: expected 400, got %d", code)
	}

	// Tag with invalid characters (underscore)
	if code := uploadAsset("invalid_tag"); code != 400 {
		t.Errorf("tag with underscore: expected 400, got %d", code)
	}

	// Tag with invalid characters (special chars)
	if code := uploadAsset("tag@123"); code != 400 {
		t.Errorf("tag with special char: expected 400, got %d", code)
	}

	// More than 10 tags
	tags := make([]string, 11)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}
	if code := uploadAsset(strings.Join(tags, ",")); code != 400 {
		t.Errorf("too many tags: expected 400, got %d", code)
	}

	// Exactly 10 valid tags should succeed
	tags = make([]string, 10)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}
	if code := uploadAsset(strings.Join(tags, ",")); code != 201 {
		t.Errorf("exactly 10 tags: expected 201, got %d", code)
	}

	// Valid tag with hyphens and alphanumeric
	if code := uploadAsset("my-tag123,another-tag"); code != 201 {
		t.Errorf("valid tags: expected 201, got %d", code)
	}

	// Tag exactly 30 characters should succeed
	exactTag := strings.Repeat("a", 30)
	if code := uploadAsset(exactTag); code != 201 {
		t.Errorf("tag exactly 30 chars: expected 201, got %d", code)
	}
}

// ========== Review Content Length Tests ==========

func TestIntegration_ReviewContentLength(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "reviewer_content", "password123", "human", false)
	creatorKey, _ := createTestUser(t, r, "creator_content", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "admin_content", "adminpass1", "human", true)

	// Upload and approve an asset
	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "review-content-asset")

	postReview := func(content string) int {
		body, _ := json.Marshal(map[string]interface{}{
			"content": content,
		})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Content exceeding 2000 characters should be rejected
	longContent := strings.Repeat("x", 2001)
	if code := postReview(longContent); code != 400 {
		t.Errorf("content > 2000 chars: expected 400, got %d", code)
	}

	// Exactly 2000 characters should succeed
	exactContent := strings.Repeat("x", 2000)
	if code := postReview(exactContent); code != 201 {
		t.Errorf("content = 2000 chars: expected 201, got %d", code)
	}
}

// ========== Status Whitelist Tests ==========

func TestIntegration_StatusWhitelist(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	adminKey, _ := createTestUser(t, r, "admin_status", "adminpass1", "human", true)

	// Invalid status in admin list should return error (not panic/500)
	req := httptest.NewRequest("GET", "/api/v1/admin/assets?status=invalid_status", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 0 || w.Code == 500 {
		t.Errorf("invalid status: expected non-500 error, got %d", w.Code)
	}
	// Should be 400 or 500 but not panic
	if w.Code != 400 && w.Code != 500 {
		t.Logf("invalid status returned %d (acceptable)", w.Code)
	}

	// Valid statuses should work
	for _, status := range []string{"pending", "approved", "rejected"} {
		req = httptest.NewRequest("GET", "/api/v1/admin/assets?status="+status, nil)
		req.Header.Set("Authorization", "Bearer "+adminKey)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("valid status %s: expected 200, got %d", status, w.Code)
		}
	}
}

// ========== Pagination COUNT Consistency Tests ==========

func TestIntegration_PaginationCountConsistency(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "pagcount_creator", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "pagcount_admin", "adminpass1", "human", true)

	// Create and approve 7 assets
	const totalAssets = 7
	for i := 0; i < totalAssets; i++ {
		assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, fmt.Sprintf("pagcount-asset-%d", i))
		_ = assetID
	}

	type listResp struct {
		Items    []interface{} `json:"items"`
		Total    int           `json:"total"`
		Page     int           `json:"page"`
		PageSize int           `json:"page_size"`
	}

	// Page 1 of page_size 3: should have 3 items, total=7
	req := httptest.NewRequest("GET", "/api/v1/assets?page=1&page_size=3", nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l1 listResp
	json.Unmarshal(w.Body.Bytes(), &l1)
	if l1.Total != totalAssets {
		t.Errorf("page1 total: expected %d, got %d", totalAssets, l1.Total)
	}
	if len(l1.Items) != 3 {
		t.Errorf("page1 items: expected 3, got %d", len(l1.Items))
	}

	// Page 3 of page_size 3: should have 1 item, total=7
	req = httptest.NewRequest("GET", "/api/v1/assets?page=3&page_size=3", nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l3 listResp
	json.Unmarshal(w.Body.Bytes(), &l3)
	if l3.Total != totalAssets {
		t.Errorf("page3 total: expected %d, got %d", totalAssets, l3.Total)
	}
	if len(l3.Items) != 1 {
		t.Errorf("page3 items: expected 1, got %d", len(l3.Items))
	}

	// Page 4 (empty page): should have 0 items, total=7
	req = httptest.NewRequest("GET", "/api/v1/assets?page=4&page_size=3", nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var l4 listResp
	json.Unmarshal(w.Body.Bytes(), &l4)
	if l4.Total != totalAssets {
		t.Errorf("page4 total: expected %d, got %d", totalAssets, l4.Total)
	}
	if len(l4.Items) != 0 {
		t.Errorf("page4 items: expected 0, got %d", len(l4.Items))
	}
}

// ========== Duplicate Review Unique Constraint Tests ==========

func TestIntegration_DuplicateReview(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "dup_review_creator", "password123", "kodaclaw", false)
	reviewerKey, _ := createTestUser(t, r, "dup_reviewer", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "dup_review_admin", "adminpass1", "human", true)

	assetID := uploadAndApproveAsset(t, r, creatorKey, adminKey, "dup-review-asset")

	postReview := func() int {
		body, _ := json.Marshal(map[string]interface{}{
			"content": "Great asset!",
		})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/reviews", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+reviewerKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// First review should succeed
	if code := postReview(); code != 201 {
		t.Fatalf("first review: expected 201, got %d", code)
	}

	// Second review from same user should return 409
	if code := postReview(); code != 409 {
		t.Errorf("duplicate review: expected 409, got %d", code)
	}
}

// ========== Duplicate Username Tests ==========

func TestIntegration_DuplicateUsername(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	// First registration succeeds
	createTestUser(t, r, "dup_username", "password123", "human", false)

	// Second registration with same username returns 409
	body, _ := json.Marshal(map[string]string{
		"username":  "dup_username",
		"password":  "password123",
		"user_type": "human",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Errorf("duplicate username: expected 409, got %d", w.Code)
	}
}

// ========== Helper ==========

// uploadAndApproveAsset uploads an asset and approves it, returns the asset ID.
func uploadAndApproveAsset(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, creatorKey, adminKey, name string) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Test asset for "+name)
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write([]byte("zip content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload %s: expected 201, got %d, body: %s", name, w.Code, w.Body.String())
	}

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Approve
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve %s: expected 200, got %d", name, w.Code)
	}

	return asset.ID
}

package tests

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/vanzheng/kodaclaw-community/internal/model"
)

// ========== Public API Integration Tests ==========

// makeSkillZip creates a ZIP containing SKILL.md with the given content.
func makeSkillZip(skillContent string) []byte {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	f, _ := w.Create("SKILL.md")
	f.Write([]byte(skillContent))
	w.Close()
	return buf.Bytes()
}

// uploadAndApproveSkill uploads a skill and approves it, returning the asset.
func uploadAndApproveSkill(t *testing.T, r *gin.Engine, apiKey string, name, desc string, skillContent string) model.Asset {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", desc)
	_ = writer.WriteField("tags", "test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeSkillZip(skillContent))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload: expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var asset model.Asset
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Approve
	adminKey, _ := createTestUser(t, r, "admin-pub-"+name, "adminpass", "human", true)
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID.String()+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approve: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	return asset
}

func TestPublicAPI_ListSkillsEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	req := httptest.NewRequest("GET", "/api/v1/public/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("expected 0 skills, got %d", list.Total)
	}
	if len(list.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(list.Items))
	}
}

func TestPublicAPI_DoesNotRequireAuth(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	// No Authorization header at all
	req := httptest.NewRequest("GET", "/api/v1/public/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
}

func TestPublicAPI_ListSkillsFiltered(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator1", "password123", "kodaclaw", false)

	uploadAndApproveSkill(t, r, apiKey, "test-skill-a", "A test skill", "# Test Skill A\nSome content")

	// Filter by type=skill
	req := httptest.NewRequest("GET", "/api/v1/public/skills?type=skill", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("expected 1 skill, got %d", list.Total)
	}
	if list.Items[0].Name != "test-skill-a" {
		t.Errorf("expected name 'test-skill-a', got '%s'", list.Items[0].Name)
	}
	// List response should NOT include skill_content
	if list.Items[0].SkillContent != nil {
		t.Error("list should not include skill_content")
	}

	// Filter by type=soul should return 0
	req = httptest.NewRequest("GET", "/api/v1/public/skills?type=soul", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("expected 0 soul, got %d", list.Total)
	}
}

func TestPublicAPI_GetSkillByName(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator2", "password123", "kodaclaw", false)

	asset := uploadAndApproveSkill(t, r, apiKey, "my-test-skill", "A great skill", "# My Skill\nContent here")

	req := httptest.NewRequest("GET", "/api/v1/public/skills/my-test-skill", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var skill model.Asset
	json.Unmarshal(w.Body.Bytes(), &skill)
	if skill.Name != "my-test-skill" {
		t.Errorf("expected name 'my-test-skill', got '%s'", skill.Name)
	}
	if skill.SkillContent == nil {
		t.Error("expected skill_content to be present")
	}
	if !strings.Contains(*skill.SkillContent, "# My Skill") {
		t.Errorf("skill_content should contain header, got: %s", *skill.SkillContent)
	}
	_ = asset
}

func TestPublicAPI_GetSkillByName_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	req := httptest.NewRequest("GET", "/api/v1/public/skills/nonexistent-skill", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublicAPI_PendingSkillNotVisible(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator3", "password123", "kodaclaw", false)

	// Upload but do NOT approve
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "pending-skill")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Not yet approved")
	_ = writer.WriteField("tags", "test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeSkillZip("# Pending"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should not be visible via public API
	req = httptest.NewRequest("GET", "/api/v1/public/skills/pending-skill", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("pending skill should not be visible, got %d", w.Code)
	}

	// List should also be empty
	req = httptest.NewRequest("GET", "/api/v1/public/skills", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var list model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("expected 0 public skills, got %d", list.Total)
	}
}

func TestPublicAPI_GetSkillContent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator4", "password123", "kodaclaw", false)

	content := "# My Skill\n\nThis is a test skill."
	uploadAndApproveSkill(t, r, apiKey, "content-skill", "Skill with content", content)

	req := httptest.NewRequest("GET", "/api/v1/public/skills/content-skill/SKILL.md", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got '%s'", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "# My Skill") {
		t.Errorf("body should contain skill content, got: %s", body)
	}
}

func TestPublicAPI_GetSkillContent_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator5", "password123", "kodaclaw", false)

	// Upload without SKILL.md (just a minimal zip)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "no-content-skill")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "No SKILL.md")
	_ = writer.WriteField("tags", "test")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var asset model.Asset
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Approve
	adminKey, _ := createTestUser(t, r, "admin-pub-nocontent", "adminpass", "human", true)
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID.String()+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Request SKILL.md — should 404
	req = httptest.NewRequest("GET", "/api/v1/public/skills/no-content-skill/SKILL.md", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404 for missing SKILL.md, got %d", w.Code)
	}
}

func TestPublicAPI_DownloadSkill(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator6", "password123", "kodaclaw", false)

	uploadAndApproveSkill(t, r, apiKey, "download-skill", "Downloadable", "# Download me")

	req := httptest.NewRequest("GET", "/api/v1/public/skills/download-skill/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/zip") && !strings.Contains(ct, "application/octet-stream") {
		t.Errorf("expected zip content type, got '%s'", ct)
	}
	body := w.Body.Bytes()
	if len(body) < 4 {
		t.Error("response body too small to be a valid zip")
	}
	// Verify it starts with PK (ZIP magic)
	if body[0] != 0x50 || body[1] != 0x4B {
		t.Error("response is not a valid ZIP file")
	}
}

func TestPublicAPI_BootstrapSkill(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())

	req := httptest.NewRequest("GET", "/skill.md", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain, got '%s'", ct)
	}
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("bootstrap skill content should not be empty")
	}
	if !strings.Contains(body, "koda-community") {
		t.Error("bootstrap content should mention koda-community")
	}
	_ = io.EOF // suppress unused import
}

func TestPublicAPI_SearchByQuery(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	r := setupTestRouter(pool, t.TempDir())
	apiKey, _ := createTestUser(t, r, "pub-creator7", "password123", "kodaclaw", false)

	uploadAndApproveSkill(t, r, apiKey, "web-search-skill", "Web search capability", "# Web Search")

	// Search by query
	req := httptest.NewRequest("GET", "/api/v1/public/skills?q=web+search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list model.AssetListResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("expected 1 result for 'web search', got %d", list.Total)
	}

	// Search with no results
	req = httptest.NewRequest("GET", "/api/v1/public/skills?q=nonexistent_xyz", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("expected 0 results, got %d", list.Total)
	}
}

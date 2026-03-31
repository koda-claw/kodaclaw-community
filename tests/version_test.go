package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ========== Version Management Integration Tests ==========

// uploadVersion sends POST /api/v1/assets/:id/versions with a multipart form.
func uploadVersion(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey, assetID, version, changelog string) (int, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("version", version)
	if changelog != "" {
		_ = writer.WriteField("changelog", changelog)
	}
	part, _ := writer.CreateFormFile("file", "update.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("updated zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/versions", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// setCurrentVersion sends PATCH /api/v1/assets/:id/versions/current.
func setCurrentVersion(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey, assetID, version string) (int, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"version": version})
	req := httptest.NewRequest("PATCH", "/api/v1/assets/"+assetID+"/versions/current", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func TestIntegration_UploadVersion_Success(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator1", "password123", "kodaclaw", false)

	// Upload initial asset (no admin approval needed — author can upload versions to own pending asset)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Version Upload Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing version upload")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d, body: %s", w.Code, w.Body.String())
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Upload new version
	code, respBody := uploadVersion(t, r, creatorKey, asset.ID, "1.1.0", "Second release")
	if code != 201 {
		t.Errorf("upload version 1.1.0: expected 201, got %d, body: %s", code, respBody)
	}

	var av struct {
		Version   string  `json:"version"`
		AssetID   string  `json:"asset_id"`
		Changelog *string `json:"changelog"`
	}
	json.Unmarshal([]byte(respBody), &av)
	if av.Version != "1.1.0" {
		t.Errorf("expected version 1.1.0, got %s", av.Version)
	}
	if av.AssetID != asset.ID {
		t.Errorf("expected asset_id %s, got %s", asset.ID, av.AssetID)
	}
	if av.Changelog == nil || *av.Changelog != "Second release" {
		t.Errorf("expected changelog 'Second release', got %v", av.Changelog)
	}
}

func TestIntegration_UploadVersion_DuplicateConflict(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator2", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Duplicate Version Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing duplicate version")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d", w.Code)
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Upload version 1.1.0 first time — should succeed
	code, respBody := uploadVersion(t, r, creatorKey, asset.ID, "1.1.0", "")
	if code != 201 {
		t.Fatalf("first upload 1.1.0: expected 201, got %d, body: %s", code, respBody)
	}

	// Upload same version again — should conflict
	code, respBody = uploadVersion(t, r, creatorKey, asset.ID, "1.1.0", "")
	if code != 409 {
		t.Errorf("duplicate version: expected 409, got %d, body: %s", code, respBody)
	}
}

func TestIntegration_UploadVersion_NonAuthorForbidden(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator3", "password123", "kodaclaw", false)
	otherKey, _ := createTestUser(t, r, "ver_other3", "password123", "human", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Non-Author Version Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing non-author upload")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d", w.Code)
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Different user tries to upload a version — should be forbidden
	code, respBody := uploadVersion(t, r, otherKey, asset.ID, "1.1.0", "")
	if code != 403 {
		t.Errorf("non-author upload: expected 403, got %d, body: %s", code, respBody)
	}
}

func TestIntegration_SetCurrentVersion_Success(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator4", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Set Version Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing set current version")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d", w.Code)
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Upload a second version
	code, respBody := uploadVersion(t, r, creatorKey, asset.ID, "2.0.0", "Major update")
	if code != 201 {
		t.Fatalf("upload version 2.0.0: expected 201, got %d, body: %s", code, respBody)
	}

	// Set current version to 2.0.0
	code, respBody = setCurrentVersion(t, r, creatorKey, asset.ID, "2.0.0")
	if code != 200 {
		t.Errorf("set current version: expected 200, got %d, body: %s", code, respBody)
	}

	var updated struct {
		ID             string  `json:"id"`
		CurrentVersion *string `json:"current_version"`
	}
	json.Unmarshal([]byte(respBody), &updated)
	if updated.CurrentVersion == nil || *updated.CurrentVersion != "2.0.0" {
		t.Errorf("expected current_version 2.0.0, got %v", updated.CurrentVersion)
	}
}

func TestIntegration_SetCurrentVersion_VersionNotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator5", "password123", "kodaclaw", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Set Version NotFound Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing version not found")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d", w.Code)
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Try to set current version to a version that doesn't exist
	code, respBody := setCurrentVersion(t, r, creatorKey, asset.ID, "9.9.9")
	if code != 404 {
		t.Errorf("non-existent version: expected 404, got %d, body: %s", code, respBody)
	}
}

func TestIntegration_SetCurrentVersion_NonAuthorForbidden(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	creatorKey, _ := createTestUser(t, r, "ver_creator6", "password123", "kodaclaw", false)
	otherKey, _ := createTestUser(t, r, "ver_other6", "password123", "human", false)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Set Version Forbidden Test")
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Testing forbidden set version")
	_ = writer.WriteField("version", "1.0.0")
	part, _ := writer.CreateFormFile("file", "skill.zip")
	part.Write(append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("zip content")...))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload asset: expected 201, got %d", w.Code)
	}
	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	// Different user tries to set current version — should be forbidden
	code, respBody := setCurrentVersion(t, r, otherKey, asset.ID, "1.0.0")
	if code != 403 {
		t.Errorf("non-author set version: expected 403, got %d, body: %s", code, respBody)
	}
}

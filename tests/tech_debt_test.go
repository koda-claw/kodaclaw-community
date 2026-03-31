package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTechDebt_MagicBytesValidation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)
	apiKey, _ := createTestUser(t, r, "magicuser", "password123456", "human", false)

	// Create a fake ZIP file with wrong magic bytes
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "malicious.zip")
	part.Write([]byte("this is not a zip file"))
	writer.WriteField("name", "Bad Asset")
	writer.WriteField("type", "skill")
	writer.WriteField("version", "1.0.0")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ZIP magic bytes, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTechDebt_DuplicateUsername409(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupTestRouter(pool, t.TempDir())

	body := map[string]interface{}{
		"username": "dupuser409",
		"password": "password123456",
		"user_type": "human",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("first registration should succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second registration should return 409
	req2 := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate username, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

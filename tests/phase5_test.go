package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// uploadZipAsset uploads a minimal ZIP asset and returns its ID.
func uploadZipAsset(t *testing.T, r http.Handler, apiKey, name, assetType, version, description, tags string) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", assetType)
	_ = writer.WriteField("description", description)
	_ = writer.WriteField("tags", tags)
	_ = writer.WriteField("version", version)
	_ = writer.WriteField("changelog", "initial")
	part, _ := writer.CreateFormFile("file", "asset.zip")
	part.Write(makeMinimalZip())
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("uploadZipAsset %q: expected 201, got %d: %s", name, w.Code, w.Body.String())
	}
	var resp struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.ID
}

// approveAssetHelper approves an asset using admin API key.
func approveAssetHelper(t *testing.T, r http.Handler, adminKey, assetID string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/admin/assets/"+assetID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("approveAssetHelper: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ===================== TestPhase5_UpdateProfile =====================

func TestPhase5_UpdateProfile(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "profile_user1", "password123", "human", false)

	t.Run("成功修改 display_name 和 description", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"display_name": "My Display Name",
			"description":  "My bio",
		})
		req := httptest.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			DisplayName *string `json:"display_name"`
			Description *string `json:"description"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.DisplayName == nil || *resp.DisplayName != "My Display Name" {
			t.Errorf("expected display_name 'My Display Name', got %v", resp.DisplayName)
		}
		if resp.Description == nil || *resp.Description != "My bio" {
			t.Errorf("expected description 'My bio', got %v", resp.Description)
		}
	})

	t.Run("只修改 display_name", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"display_name": "Only Name"})
		req := httptest.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			DisplayName *string `json:"display_name"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.DisplayName == nil || *resp.DisplayName != "Only Name" {
			t.Errorf("expected 'Only Name', got %v", resp.DisplayName)
		}
	})

	t.Run("GetMe 返回更新后的值", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"display_name": "Final Name",
			"description":  "Final bio",
		})
		req := httptest.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("PATCH expected 200, got %d", w.Code)
		}

		req2 := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		req2.Header.Set("Authorization", "Bearer "+apiKey)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Fatalf("GET /users/me expected 200, got %d", w2.Code)
		}
		var me struct {
			DisplayName *string `json:"display_name"`
			Description *string `json:"description"`
		}
		json.Unmarshal(w2.Body.Bytes(), &me)
		if me.DisplayName == nil || *me.DisplayName != "Final Name" {
			t.Errorf("GetMe display_name: expected 'Final Name', got %v", me.DisplayName)
		}
		if me.Description == nil || *me.Description != "Final bio" {
			t.Errorf("GetMe description: expected 'Final bio', got %v", me.Description)
		}
	})
}

// ===================== TestPhase5_ChangePassword =====================

func TestPhase5_ChangePassword(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "pw_user1", "oldpassword", "human", false)

	t.Run("旧密码错误时返回 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"old_password": "wrongpassword",
			"new_password": "newpassword123",
		})
		req := httptest.NewRequest("PATCH", "/api/v1/auth/password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("新密码过短时返回 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"old_password": "oldpassword",
			"new_password": "short",
		})
		req := httptest.NewRequest("PATCH", "/api/v1/auth/password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("旧密码正确时成功修改", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"old_password": "oldpassword",
			"new_password": "newpassword123",
		})
		req := httptest.NewRequest("PATCH", "/api/v1/auth/password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// New password works via login
		loginBody, _ := json.Marshal(map[string]string{
			"username": "pw_user1",
			"password": "newpassword123",
		})
		req2 := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Errorf("login with new password: expected 200, got %d", w2.Code)
		}

		// Old password no longer works
		loginBody2, _ := json.Marshal(map[string]string{
			"username": "pw_user1",
			"password": "oldpassword",
		})
		req3 := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody2))
		req3.Header.Set("Content-Type", "application/json")
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		if w3.Code != 401 {
			t.Errorf("login with old password: expected 401, got %d", w3.Code)
		}
	})
}

// ===================== TestPhase5_AssetUpdate =====================

func TestPhase5_AssetUpdate(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "asset_author1", "password123", "kodaclaw", false)
	otherKey, _ := createTestUser(t, r, "asset_other1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "asset_admin1", "adminpass1", "human", true)

	pendingID := uploadZipAsset(t, r, authorKey, "Original Name", "skill", "1.0.0", "Original description", "go,test")

	approvedID := uploadZipAsset(t, r, authorKey, "Approved Asset", "skill", "1.0.0", "Will be approved", "go")
	approveAssetHelper(t, r, adminKey, approvedID)

	t.Run("作者成功更新资产名称和描述", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":        "Updated Name",
			"description": "Updated description",
			"tags":        []string{"go", "updated"},
		})
		req := httptest.NewRequest("PATCH", "/api/v1/assets/"+pendingID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Name != "Updated Name" {
			t.Errorf("expected 'Updated Name', got %q", resp.Name)
		}
		if resp.Description != "Updated description" {
			t.Errorf("expected 'Updated description', got %q", resp.Description)
		}
	})

	t.Run("更新已 approved 的资产状态变为 pending", func(t *testing.T) {
		// Verify approved
		req := httptest.NewRequest("GET", "/api/v1/assets/"+approvedID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var before struct{ Status string `json:"status"` }
		json.Unmarshal(w.Body.Bytes(), &before)
		if before.Status != "approved" {
			t.Fatalf("expected status=approved before update, got %q", before.Status)
		}

		body, _ := json.Marshal(map[string]interface{}{
			"name":        "Approved Asset Updated",
			"description": "Updated after approval",
			"tags":        []string{"go"},
		})
		req2 := httptest.NewRequest("PATCH", "/api/v1/assets/"+approvedID, bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+authorKey)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
		}
		var after struct{ Status string `json:"status"` }
		json.Unmarshal(w2.Body.Bytes(), &after)
		if after.Status != "pending" {
			t.Errorf("expected status=pending after update, got %q", after.Status)
		}
	})

	t.Run("非作者尝试更新返回 403", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":        "Hacker Name",
			"description": "Hacked",
			"tags":        []string{"hack"},
		})
		req := httptest.NewRequest("PATCH", "/api/v1/assets/"+pendingID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("tag 超长返回 400", func(t *testing.T) {
		longTag := "averylongtagthatexceedsthirtycharacterslimit"
		body, _ := json.Marshal(map[string]interface{}{
			"name":        "Test",
			"description": "Test",
			"tags":        []string{longTag},
		})
		req := httptest.NewRequest("PATCH", "/api/v1/assets/"+pendingID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("expected 400 for long tag, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("tag 含非法字符返回 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":        "Test",
			"description": "Test",
			"tags":        []string{"invalid tag!"},
		})
		req := httptest.NewRequest("PATCH", "/api/v1/assets/"+pendingID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("expected 400 for invalid tag chars, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// ===================== TestPhase5_AssetDelete =====================

func TestPhase5_AssetDelete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "del_author1", "password123", "kodaclaw", false)
	otherKey, _ := createTestUser(t, r, "del_other1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "del_admin1", "adminpass1", "human", true)

	t.Run("非作者尝试删除返回 403", func(t *testing.T) {
		assetID := uploadZipAsset(t, r, authorKey, "Asset To Protect", "skill", "1.0.0", "Protected asset", "go")
		req := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+otherKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("作者成功删除资产", func(t *testing.T) {
		assetID := uploadZipAsset(t, r, authorKey, "Asset To Delete", "skill", "1.0.0", "Will be deleted", "go")
		req := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("删除后再获取返回 404", func(t *testing.T) {
		assetID := uploadZipAsset(t, r, authorKey, "Asset 404", "skill", "1.0.0", "Will 404", "go")

		req := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("delete: expected 200, got %d", w.Code)
		}

		req2 := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
		req2.Header.Set("Authorization", "Bearer "+authorKey)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 404 {
			t.Errorf("after delete GET: expected 404, got %d", w2.Code)
		}
	})

	t.Run("删除后资产不出现在列表中", func(t *testing.T) {
		assetID := uploadZipAsset(t, r, authorKey, "Listed Then Deleted", "skill", "1.0.0", "In list briefly", "go")
		approveAssetHelper(t, r, adminKey, assetID)

		// Verify it's in the list before delete
		req := httptest.NewRequest("GET", "/api/v1/assets", nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var listBefore struct{ Total int `json:"total"` }
		json.Unmarshal(w.Body.Bytes(), &listBefore)
		if listBefore.Total == 0 {
			t.Fatal("expected at least 1 asset before delete")
		}

		// Clean up notifications referencing this asset (no ON DELETE CASCADE on related_asset_id)
		pool.Exec(context.Background(), "DELETE FROM notifications WHERE related_asset_id = $1", assetID)

		// Delete
		req2 := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID, nil)
		req2.Header.Set("Authorization", "Bearer "+authorKey)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Fatalf("delete: expected 200, got %d", w2.Code)
		}

		// Verify removed from list
		req3 := httptest.NewRequest("GET", "/api/v1/assets", nil)
		req3.Header.Set("Authorization", "Bearer "+authorKey)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		var listAfter struct {
			Items []struct{ ID string `json:"id"` } `json:"items"`
		}
		json.Unmarshal(w3.Body.Bytes(), &listAfter)
		for _, item := range listAfter.Items {
			if item.ID == assetID {
				t.Errorf("deleted asset %s still appears in list", assetID)
			}
		}
	})
}

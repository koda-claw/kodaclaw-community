package tests

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// createZipWithDocs builds a valid ZIP containing README.md and/or SKILL.md.
func createZipWithDocs(readmeContent, skillContent string) []byte {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	if readmeContent != "" {
		f, _ := w.Create("README.md")
		f.Write([]byte(readmeContent))
	}
	if skillContent != "" {
		f, _ := w.Create("SKILL.md")
		f.Write([]byte(skillContent))
	}
	w.Close()
	return buf.Bytes()
}

// uploadZipAssetFull uploads a ZIP asset, optionally with embedded docs.
func uploadZipAssetFull(t *testing.T, r http.Handler, apiKey, name, assetType, version, description string, zipBytes []byte) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", assetType)
	_ = writer.WriteField("description", description)
	_ = writer.WriteField("version", version)
	_ = writer.WriteField("changelog", "initial")
	part, _ := writer.CreateFormFile("file", "asset.zip")
	part.Write(zipBytes)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/assets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("uploadZipAssetFull %q: expected 201, got %d: %s", name, w.Code, w.Body.String())
	}
	var resp struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.ID
}

// ==================== TestPhase5b_AssetDependencies ====================

func TestPhase5b_AssetDependencies(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "dep_author", "password123", "human", false)
	otherKey, _ := createTestUser(t, r, "dep_other", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "dep_admin", "password123", "human", true)

	minZip := makeMinimalZip()
	assetID := uploadZipAssetFull(t, r, authorKey, "Main Asset", "skill", "1.0.0", "main asset", minZip)
	depID := uploadZipAssetFull(t, r, authorKey, "Dep Asset", "skill", "1.0.0", "dependency", minZip)
	approveAssetHelper(t, r, adminKey, assetID)
	approveAssetHelper(t, r, adminKey, depID)

	t.Run("作者添加依赖", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"asset_id": depID})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/dependencies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 201 {
			t.Fatalf("add dependency: expected 201, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("获取依赖列表", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/dependencies", nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("list dependencies: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var deps []map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &deps)
		if len(deps) != 1 {
			t.Fatalf("expected 1 dependency, got %d", len(deps))
		}
		if deps[0]["depends_on_asset_id"] != depID {
			t.Errorf("expected dep id %s, got %v", depID, deps[0]["depends_on_asset_id"])
		}
	})

	t.Run("非作者不能添加依赖", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"asset_id": depID})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/dependencies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("non-author add dep: expected 403, got %d", w.Code)
		}
	})

	t.Run("重复添加依赖返回 409", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"asset_id": depID})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/dependencies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 409 {
			t.Errorf("duplicate dep: expected 409, got %d", w.Code)
		}
	})

	t.Run("自我依赖返回 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"asset_id": assetID})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/dependencies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("self dep: expected 400, got %d", w.Code)
		}
	})

	t.Run("作者删除依赖", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID+"/dependencies/"+depID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("delete dep: expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("删除后依赖列表为空", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID+"/dependencies", nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var deps []map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &deps)
		if len(deps) != 0 {
			t.Errorf("expected 0 deps after delete, got %d", len(deps))
		}
	})

	t.Run("非作者不能删除依赖", func(t *testing.T) {
		// Re-add first
		body, _ := json.Marshal(map[string]string{"asset_id": depID})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/dependencies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		req2 := httptest.NewRequest("DELETE", "/api/v1/assets/"+assetID+"/dependencies/"+depID, nil)
		req2.Header.Set("Authorization", "Bearer "+otherKey)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != 403 {
			t.Errorf("non-author delete dep: expected 403, got %d", w2.Code)
		}
	})
}

// ==================== TestPhase5b_ContentPreview ====================

func TestPhase5b_ContentPreview(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "preview_author", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "preview_admin", "password123", "human", true)

	t.Run("上传含 README.md 的资产后可获取内容", func(t *testing.T) {
		readmeText := "# My Asset\nThis is the readme."
		skillText := "## Skill\nHow to use this skill."
		zipBytes := createZipWithDocs(readmeText, skillText)

		assetID := uploadZipAssetFull(t, r, authorKey, "Preview Asset", "skill", "1.0.0", "preview test", zipBytes)
		approveAssetHelper(t, r, adminKey, assetID)

		req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("get asset: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["readme"] == nil {
			t.Error("expected readme field, got nil")
		} else if resp["readme"].(string) != readmeText {
			t.Errorf("readme mismatch: got %q", resp["readme"])
		}

		if resp["skill_content"] == nil {
			t.Error("expected skill_content field, got nil")
		} else if resp["skill_content"].(string) != skillText {
			t.Errorf("skill_content mismatch: got %q", resp["skill_content"])
		}
	})

	t.Run("上传不含 README.md 的资产后字段不返回", func(t *testing.T) {
		minZip := makeMinimalZip()
		assetID := uploadZipAssetFull(t, r, authorKey, "No Readme Asset", "skill", "1.0.0", "no readme", minZip)
		approveAssetHelper(t, r, adminKey, assetID)

		req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+authorKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["readme"] != nil {
			t.Errorf("expected no readme field, got %v", resp["readme"])
		}
		if resp["skill_content"] != nil {
			t.Errorf("expected no skill_content field, got %v", resp["skill_content"])
		}
	})
}

// ==================== TestPhase5b_InstallCount ====================

func TestPhase5b_InstallCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	authorKey, _ := createTestUser(t, r, "install_author", "password123", "human", false)
	user1Key, _ := createTestUser(t, r, "install_user1", "password123", "human", false)
	user2Key, _ := createTestUser(t, r, "install_user2", "password123", "human", false)
	adminKey, _ := createTestUser(t, r, "install_admin", "password123", "human", true)

	minZip := makeMinimalZip()
	assetID := uploadZipAssetFull(t, r, authorKey, "Install Asset", "skill", "1.0.0", "install test", minZip)
	approveAssetHelper(t, r, adminKey, assetID)

	installAsset := func(t *testing.T, apiKey, instanceID string) int {
		t.Helper()
		var body []byte
		if instanceID != "" {
			body, _ = json.Marshal(map[string]string{"instance_id": instanceID})
		} else {
			body, _ = json.Marshal(map[string]string{})
		}
		req := httptest.NewRequest("POST", "/api/v1/assets/"+assetID+"/install", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	getInstallCount := func(t *testing.T, apiKey string) int {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/assets/"+assetID, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp struct {
			InstallCount int `json:"install_count"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.InstallCount
	}

	t.Run("用户安装资产成功", func(t *testing.T) {
		code := installAsset(t, user1Key, "")
		if code != 200 {
			t.Fatalf("install: expected 200, got %d", code)
		}
	})

	t.Run("安装后 install_count 增加到 1", func(t *testing.T) {
		count := getInstallCount(t, user1Key)
		if count != 1 {
			t.Errorf("expected install_count=1, got %d", count)
		}
	})

	t.Run("同一用户重复安装不增加计数", func(t *testing.T) {
		installAsset(t, user1Key, "")
		count := getInstallCount(t, user1Key)
		if count != 1 {
			t.Errorf("expected install_count=1 after duplicate, got %d", count)
		}
	})

	t.Run("不同用户安装增加计数", func(t *testing.T) {
		installAsset(t, user2Key, "")
		count := getInstallCount(t, user1Key)
		if count != 2 {
			t.Errorf("expected install_count=2 after second user, got %d", count)
		}
	})

	t.Run("同用户不同 instance_id 视为不同安装", func(t *testing.T) {
		installAsset(t, user1Key, "instance-abc")
		count := getInstallCount(t, user1Key)
		if count != 3 {
			t.Errorf("expected install_count=3 after instance install, got %d", count)
		}
	})

	t.Run("List 接口也包含 install_count", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/assets", nil)
		req.Header.Set("Authorization", "Bearer "+user1Key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("list assets: expected 200, got %d", w.Code)
		}
		var resp struct {
			Items []struct {
				ID           string `json:"id"`
				InstallCount int    `json:"install_count"`
			} `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		for _, item := range resp.Items {
			if item.ID == assetID {
				if item.InstallCount != 3 {
					t.Errorf("list: expected install_count=3, got %d", item.InstallCount)
				}
				return
			}
		}
		t.Error("asset not found in list")
	})

	t.Run("未审批资产不能安装", func(t *testing.T) {
		pendingID := uploadZipAssetFull(t, r, authorKey, "Pending Asset", "skill", "1.0.0", "pending", minZip)
		code := installAsset(t, user1Key, "")
		_ = pendingID
		// Install on the pending asset (different id - use pendingID)
		body, _ := json.Marshal(map[string]string{})
		req := httptest.NewRequest("POST", "/api/v1/assets/"+pendingID+"/install", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user1Key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Errorf("pending asset install: expected 404, got %d (previous code %d)", w.Code, code)
		}
	})
}

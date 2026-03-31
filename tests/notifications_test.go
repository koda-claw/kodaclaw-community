package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func uploadAndRejectAsset(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, creatorKey, adminKey, name, reason string) string {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("type", "skill")
	_ = writer.WriteField("description", "Test asset for "+name)
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
		t.Fatalf("upload %s: expected 201, got %d, body: %s", name, w.Code, w.Body.String())
	}

	var asset struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &asset)

	rejectBody, _ := json.Marshal(map[string]string{"reason": reason})
	req = httptest.NewRequest("POST", "/api/v1/admin/assets/"+asset.ID+"/reject", bytes.NewReader(rejectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("reject %s: expected 200, got %d, body: %s", name, w.Code, w.Body.String())
	}

	return asset.ID
}

func listNotifications(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, apiKey string, unreadOnly bool) (int, int, []map[string]interface{}) {
	t.Helper()
	url := "/api/v1/users/me/notifications"
	if unreadOnly {
		url += "?unread=true"
	}
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list notifications: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var result struct {
		Items  []map[string]interface{} `json:"items"`
		Total  int                      `json:"total"`
		Unread int                      `json:"unread"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result.Total, result.Unread, result.Items
}

func TestIntegration_Notifications_CreatedOnApproval(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator1", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin1", "password123", "human", true)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif Approval Asset")

	total, unread, items := listNotifications(t, r, creatorKey, false)
	if total != 1 {
		t.Errorf("expected 1 notification, got %d", total)
	}
	if unread != 1 {
		t.Errorf("expected 1 unread, got %d", unread)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["type"] != "asset_approved" {
		t.Errorf("expected type=asset_approved, got %v", items[0]["type"])
	}
}

func TestIntegration_Notifications_CreatedOnRejection(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator2", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin2", "password123", "human", true)

	uploadAndRejectAsset(t, r, creatorKey, adminKey, "Notif Rejection Asset", "质量不达标")

	total, unread, items := listNotifications(t, r, creatorKey, false)
	if total != 1 {
		t.Errorf("expected 1 notification, got %d", total)
	}
	if unread != 1 {
		t.Errorf("expected 1 unread, got %d", unread)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["type"] != "asset_rejected" {
		t.Errorf("expected type=asset_rejected, got %v", items[0]["type"])
	}
}

func TestIntegration_Notifications_ListEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	userKey, _ := createTestUser(t, r, "notif_user3", "password123", "human", false)

	total, unread, items := listNotifications(t, r, userKey, false)
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if unread != 0 {
		t.Errorf("expected unread=0, got %d", unread)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestIntegration_Notifications_ListNonEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator4", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin4", "password123", "human", true)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif List Asset One")
	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif List Asset Two")

	total, _, items := listNotifications(t, r, creatorKey, false)
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestIntegration_Notifications_ListOnlyUnread(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator5", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin5", "password123", "human", true)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif Unread Asset One")
	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif Unread Asset Two")

	// Get all notifications, find the first one's ID
	_, _, items := listNotifications(t, r, creatorKey, false)
	if len(items) < 2 {
		t.Fatalf("expected at least 2 notifications, got %d", len(items))
	}
	firstID, _ := items[0]["id"].(string)

	// Mark first notification as read
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/notifications/"+firstID, nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read: expected 200, got %d", w.Code)
	}

	// List only unread: should have 1
	total, unread, unreadItems := listNotifications(t, r, creatorKey, true)
	if total != 1 {
		t.Errorf("expected total=1 unread, got %d", total)
	}
	if unread != 1 {
		t.Errorf("expected unread count=1, got %d", unread)
	}
	if len(unreadItems) != 1 {
		t.Errorf("expected 1 unread item, got %d", len(unreadItems))
	}
}

func TestIntegration_Notifications_MarkRead(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator6", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin6", "password123", "human", true)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif Mark Read Asset")

	_, _, items := listNotifications(t, r, creatorKey, false)
	if len(items) == 0 {
		t.Fatal("expected at least 1 notification")
	}
	nid, _ := items[0]["id"].(string)

	// Initially unread
	if items[0]["is_read"] != false {
		t.Error("expected is_read=false initially")
	}

	// Mark as read
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/notifications/"+nid, nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Verify now read
	_, unread, _ := listNotifications(t, r, creatorKey, false)
	if unread != 0 {
		t.Errorf("expected unread=0 after marking read, got %d", unread)
	}
}

func TestIntegration_Notifications_MarkAllRead(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator7", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin7", "password123", "human", true)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif MarkAll Asset One")
	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif MarkAll Asset Two")
	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif MarkAll Asset Three")

	_, unreadBefore, _ := listNotifications(t, r, creatorKey, false)
	if unreadBefore != 3 {
		t.Errorf("expected 3 unread before, got %d", unreadBefore)
	}

	// Mark all as read
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/notifications/read-all", nil)
	req.Header.Set("Authorization", "Bearer "+creatorKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark all read: expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	_, unreadAfter, _ := listNotifications(t, r, creatorKey, false)
	if unreadAfter != 0 {
		t.Errorf("expected 0 unread after mark-all-read, got %d", unreadAfter)
	}
}

func TestIntegration_Notifications_NotAccessibleByOtherUser(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r := setupTestRouter(pool, tmpDir)

	creatorKey, _ := createTestUser(t, r, "notif_creator8", "password123", "kodaclaw", false)
	adminKey, _ := createTestUser(t, r, "notif_admin8", "password123", "human", true)
	otherKey, _ := createTestUser(t, r, "notif_other8", "password123", "human", false)

	uploadAndApproveAsset(t, r, creatorKey, adminKey, "Notif Other User Asset")

	// Creator has the notification
	total, _, items := listNotifications(t, r, creatorKey, false)
	if total != 1 {
		t.Errorf("creator: expected 1 notification, got %d", total)
	}
	nid, _ := items[0]["id"].(string)

	// Other user has no notifications
	otherTotal, _, _ := listNotifications(t, r, otherKey, false)
	if otherTotal != 0 {
		t.Errorf("other user: expected 0 notifications, got %d", otherTotal)
	}

	// Other user cannot mark creator's notification as read (should 404)
	req := httptest.NewRequest("PATCH", "/api/v1/users/me/notifications/"+nid, nil)
	req.Header.Set("Authorization", "Bearer "+otherKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("other user marking creator's notification: expected 404, got %d", w.Code)
	}
}

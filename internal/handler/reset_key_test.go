package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// mockUserRepoForReset implements repository.UserRepository for ResetKeyHandler tests.
// Methods not used by ResetKeyHandler return zero values / nil.
type mockUserRepoForReset struct {
	getByIDFn              func(ctx context.Context, id uuid.UUID) (*model.User, error)
	getByAPIKeyFn          func(ctx context.Context, apiKey string) (*model.User, error)
	getByUsernameFn        func(ctx context.Context, username string) (*model.User, error)
	getUserByResetTokenFn  func(ctx context.Context, token string) (*model.User, error)
	updateResetTokenFn     func(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error
	clearResetTokenFn      func(ctx context.Context, userID uuid.UUID) error
	updateAPIKeyFn         func(ctx context.Context, userID uuid.UUID, newKey string) error
}

func (m *mockUserRepoForReset) Create(ctx context.Context, user *model.User) error {
	return nil
}
func (m *mockUserRepoForReset) CreateWithGitHub(ctx context.Context, user *model.User) error {
	return nil
}
func (m *mockUserRepoForReset) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if m.getByUsernameFn != nil {
		return m.getByUsernameFn(ctx, username)
	}
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) GetByAPIKey(ctx context.Context, apiKey string) (*model.User, error) {
	if m.getByAPIKeyFn != nil {
		return m.getByAPIKeyFn(ctx, apiKey)
	}
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) GetByGitHubID(ctx context.Context, githubID int64) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) UpdateProfile(ctx context.Context, id uuid.UUID, displayName, description *string) error {
	return nil
}
func (m *mockUserRepoForReset) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	return nil
}
func (m *mockUserRepoForReset) UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL string) error {
	return nil
}
func (m *mockUserRepoForReset) GetByBindCode(ctx context.Context, code string) (*model.User, error) {
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) BindObserver(ctx context.Context, kodaclawUserID, observerUserID uuid.UUID) error {
	return nil
}
func (m *mockUserRepoForReset) GetObservedInstance(ctx context.Context, observerUserID uuid.UUID) ([]model.User, error) {
	return nil, nil
}
func (m *mockUserRepoForReset) Count(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *mockUserRepoForReset) CountByDay(ctx context.Context, days int) ([]repository.DayCount, error) {
	return nil, nil
}
func (m *mockUserRepoForReset) UpdateResetToken(ctx context.Context, userID uuid.UUID, token string, expires time.Time) error {
	if m.updateResetTokenFn != nil {
		return m.updateResetTokenFn(ctx, userID, token, expires)
	}
	return nil
}
func (m *mockUserRepoForReset) GetUserByResetToken(ctx context.Context, token string) (*model.User, error) {
	if m.getUserByResetTokenFn != nil {
		return m.getUserByResetTokenFn(ctx, token)
	}
	return nil, repository.ErrUserNotFound
}
func (m *mockUserRepoForReset) ClearResetToken(ctx context.Context, userID uuid.UUID) error {
	if m.clearResetTokenFn != nil {
		return m.clearResetTokenFn(ctx, userID)
	}
	return nil
}
func (m *mockUserRepoForReset) UpdateAPIKey(ctx context.Context, userID uuid.UUID, newKey string) error {
	if m.updateAPIKeyFn != nil {
		return m.updateAPIKeyFn(ctx, userID, newKey)
	}
	return nil
}

func TestResetKeyRequest_Success(t *testing.T) {
	ghID := int64(12345)
	userID := uuid.New()

	repo := &mockUserRepoForReset{
		getByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			if username != "testuser" {
				return nil, repository.ErrUserNotFound
			}
			return &model.User{
				ID:             userID,
				Username:       "testuser",
				GitHubID:       &ghID,
				GitHubUsername: strPtr("testgh"),
			}, nil
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{"username": "testuser"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/request", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyRequest(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	oauthURL, _ := resp["github_oauth_url"].(string)
	if oauthURL == "" {
		t.Error("expected non-empty github_oauth_url in response")
	}
	if oauthURL != "/api/v1/auth/github?state=/reset-key/testuser" {
		t.Errorf("expected github_oauth_url to contain /reset-key/testuser, got %s", oauthURL)
	}
}

func TestResetKeyRequest_GitHubNotBound(t *testing.T) {
	userID := uuid.New()

	repo := &mockUserRepoForReset{
		getByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return &model.User{
				ID:       userID,
				Username: "testuser",
			}, nil
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{"username": "testuser"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/request", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyRequest(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "GITHUB_REQUIRED" {
		t.Errorf("expected error GITHUB_REQUIRED, got %v", resp["error"])
	}
	oauthURL, _ := resp["github_oauth_url"].(string)
	if oauthURL == "" {
		t.Error("expected github_oauth_url in GITHUB_REQUIRED response")
	}
}

func TestResetKeyRequest_UserNotFound(t *testing.T) {
	repo := &mockUserRepoForReset{
		getByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return nil, repository.ErrUserNotFound
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{"username": "nonexistent"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/request", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyRequest(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestResetKeyConfirm_Success(t *testing.T) {
	ghID := int64(12345)
	userID := uuid.New()
	expires := time.Now().Add(24 * time.Hour)
	var updatedKey string

	repo := &mockUserRepoForReset{
		getUserByResetTokenFn: func(ctx context.Context, token string) (*model.User, error) {
			if token != "valid-reset-token" {
				return nil, repository.ErrUserNotFound
			}
			return &model.User{
				ID:                 userID,
				Username:           "testuser",
				GitHubID:           &ghID,
				APIKeyResetToken:   strPtr("valid-reset-token"),
				APIKeyResetExpires: &expires,
			}, nil
		},
		updateAPIKeyFn: func(ctx context.Context, id uuid.UUID, newKey string) error {
			updatedKey = newKey
			return nil
		},
		clearResetTokenFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"reset_token": "valid-reset-token",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/confirm", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyConfirm(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["api_key"] == nil || resp["api_key"].(string) == "" {
		t.Error("expected non-empty api_key in response")
	}
	if updatedKey == "" {
		t.Error("expected API key to be updated in repo")
	}
}

func TestResetKeyConfirm_ExpiredToken(t *testing.T) {
	ghID := int64(12345)
	userID := uuid.New()
	expires := time.Now().Add(-1 * time.Hour) // expired

	repo := &mockUserRepoForReset{
		getUserByResetTokenFn: func(ctx context.Context, token string) (*model.User, error) {
			return &model.User{
				ID:                 userID,
				Username:           "testuser",
				GitHubID:           &ghID,
				APIKeyResetToken:   strPtr("expired-token"),
				APIKeyResetExpires: &expires,
			}, nil
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"reset_token": "expired-token",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/confirm", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyConfirm(c)

	if w.Code != http.StatusGone {
		t.Errorf("expected status 410, got %d", w.Code)
	}
}

func TestResetKeyConfirm_InvalidToken(t *testing.T) {
	repo := &mockUserRepoForReset{
		getUserByResetTokenFn: func(ctx context.Context, token string) (*model.User, error) {
			return nil, repository.ErrUserNotFound
		},
	}

	h := NewResetKeyHandler(repo)

	body, _ := json.Marshal(map[string]string{
		"reset_token": "nonexistent",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key/confirm", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetKeyConfirm(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleResetKeyCallback(t *testing.T) {
	ghID := int64(12345)
	userID := uuid.New()
	var savedToken string
	var savedExpires time.Time

	repo := &mockUserRepoForReset{
		updateResetTokenFn: func(ctx context.Context, id uuid.UUID, token string, expires time.Time) error {
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			savedToken = token
			savedExpires = expires
			return nil
		},
	}

	h := NewResetKeyHandler(repo)

	user := &model.User{
		ID:       userID,
		Username: "testuser",
		GitHubID: &ghID,
	}

	token, err := h.HandleResetKeyCallback(context.Background(), user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty reset token")
	}
	if savedToken != token {
		t.Error("saved token mismatch")
	}
	if savedExpires.Before(time.Now()) {
		t.Error("expected expiry to be in the future")
	}
}

func strPtr(s string) *string {
	return &s
}

func TestResetKeyDirect_Success(t *testing.T) {
	userID := uuid.New()
	var updatedKey string
	var updatedID uuid.UUID

	repo := &mockUserRepoForReset{
		updateAPIKeyFn: func(ctx context.Context, id uuid.UUID, newKey string) error {
			updatedID = id
			updatedKey = newKey
			return nil
		},
	}

	h := NewResetKeyHandler(repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key", nil)
	c.Set(middleware.ContextUserID, userID.String())

	h.ResetKeyDirect(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	apiKey, ok := resp["api_key"].(string)
	if !ok || apiKey == "" {
		t.Errorf("expected non-empty api_key in response, got %v", resp["api_key"])
	}
	if len(apiKey) != 32 {
		t.Errorf("expected api_key length 32, got %d", len(apiKey))
	}
	if updatedKey == "" {
		t.Error("expected updateAPIKeyFn to be called with non-empty key")
	}
	if updatedID != userID {
		t.Errorf("expected updateAPIKeyFn called with userID %s, got %s", userID, updatedID)
	}
}

func TestResetKeyDirect_DBError(t *testing.T) {
	userID := uuid.New()

	repo := &mockUserRepoForReset{
		updateAPIKeyFn: func(ctx context.Context, id uuid.UUID, newKey string) error {
			return fmt.Errorf("database error")
		},
	}

	h := NewResetKeyHandler(repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/auth/reset-key", nil)
	c.Set(middleware.ContextUserID, userID.String())

	h.ResetKeyDirect(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d; body: %s", w.Code, w.Body.String())
	}
}

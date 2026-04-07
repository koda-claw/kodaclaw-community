package tests

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/vanzheng/kodaclaw-community/internal/handler"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"github.com/vanzheng/kodaclaw-community/internal/router"
	"github.com/vanzheng/kodaclaw-community/internal/service"
)

func TestIntegration_RelayWebhook_StrictAccepted(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r, hub := setupRelayWebhookTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "relay_webhook_user1", "password123", "human", false)
	instance := createRelayInstanceForTest(t, r, apiKey, "Strict Relay")

	client := relay.NewClient(hub, nil)
	hub.Register(instance.AccountID, client)

	body := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"monitor:btc:4h",
		"externalMessageId":"msg-2026-04-07T02:00:00Z",
		"text":"BTC moved 1.5%",
		"sender":{"id":"crypto-monitor","displayName":"Crypto Monitor","isBot":true},
		"occurredAt":"2026-04-07T02:00:00Z",
		"correlationId":"run-123",
		"payload":{"symbol":"BTC"}
	}`)
	timestamp := time.Now().Unix()
	signature := signRelayWebhook(instance.WebhookSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/"+instance.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Relay-Signature", signature)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Accepted bool   `json:"accepted"`
		EventID  string `json:"eventId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("expected accepted=true, got false")
	}
	if resp.EventID == "" {
		t.Fatal("expected eventId to be populated")
	}

	select {
	case queued := <-client.Outbox():
		frame, ok := queued.(relay.EventFrame)
		if !ok {
			t.Fatalf("expected EventFrame, got %T", queued)
		}
		if frame.EventType != "MessageReceived" {
			t.Fatalf("unexpected eventType: %q", frame.EventType)
		}
		if frame.ThreadType != "DirectMessage" {
			t.Fatalf("unexpected threadType: %q", frame.ThreadType)
		}
		if frame.ExternalThreadID != "monitor:btc:4h" {
			t.Fatalf("unexpected externalThreadId: %q", frame.ExternalThreadID)
		}
		if frame.ExternalMessageID != "msg-2026-04-07T02:00:00Z" {
			t.Fatalf("unexpected externalMessageId: %q", frame.ExternalMessageID)
		}
		if frame.Sender.ID != "crypto-monitor" || !frame.Sender.IsBot {
			t.Fatalf("unexpected sender: %+v", frame.Sender)
		}
		if frame.CorrelationID != "run-123" {
			t.Fatalf("unexpected correlationId: %q", frame.CorrelationID)
		}
	default:
		t.Fatal("expected webhook event to be queued to relay hub")
	}
}

func TestIntegration_RelayWebhook_StrictRejectsLegacyPayload(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r, _ := setupRelayWebhookTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "relay_webhook_user2", "password123", "human", false)
	instance := createRelayInstanceForTest(t, r, apiKey, "Legacy Reject")

	body := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"monitor:btc:4h",
		"externalMessageId":"msg-legacy",
		"senderId":"crypto-monitor",
		"senderName":"Crypto Monitor",
		"occurredAt":"2026-04-07T02:00:00Z"
	}`)
	timestamp := time.Now().Unix()
	signature := signRelayWebhook(instance.WebhookSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/"+instance.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Relay-Signature", signature)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Accepted  bool   `json:"accepted"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Accepted {
		t.Fatal("expected accepted=false")
	}
	if resp.ErrorCode == "" {
		t.Fatal("expected errorCode to be populated")
	}
}

func TestIntegration_RelayWebhook_KeyIDAccepted(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	tmpDir := t.TempDir()
	r, hub := setupRelayWebhookTestRouter(pool, tmpDir)

	apiKey, _ := createTestUser(t, r, "relay_webhook_user3", "password123", "human", false)
	instance := createRelayInstanceForTest(t, r, apiKey, "KeyID Relay")
	key := createRelayWebhookKeyForTest(t, r, apiKey, instance.ID, "secondary")

	client := relay.NewClient(hub, nil)
	hub.Register(instance.AccountID, client)

	body := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"NotificationReceived",
		"threadType":"Group",
		"externalThreadId":"alerts:ops",
		"externalMessageId":"notif-001",
		"text":"CPU alert triggered",
		"sender":{"id":"ops-bot","displayName":"Ops Bot","isBot":true},
		"occurredAt":"2026-04-07T03:00:00Z"
	}`)
	timestamp := time.Now().Unix()
	signature := signRelayWebhook(key.KeyValue, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/"+instance.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Relay-Signature", signature)
	req.Header.Set("X-Relay-KeyId", key.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	select {
	case queued := <-client.Outbox():
		frame := queued.(relay.EventFrame)
		if frame.EventType != "NotificationReceived" {
			t.Fatalf("unexpected eventType: %q", frame.EventType)
		}
		if frame.ExternalMessageID != "notif-001" {
			t.Fatalf("unexpected externalMessageId: %q", frame.ExternalMessageID)
		}
	default:
		t.Fatal("expected event to be queued when using X-Relay-KeyId")
	}
}

func TestIntegration_RelayWebhook_StoresOfflineEventInRedis(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping failed: %v", err)
	}
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("redis flush failed: %v", err)
	}

	tmpDir := t.TempDir()
	r, _ := setupRelayWebhookTestRouterWithRedis(pool, tmpDir, rdb)

	apiKey, _ := createTestUser(t, r, "relay_webhook_user4", "password123", "human", false)
	instance := createRelayInstanceForTest(t, r, apiKey, "Offline Relay")

	body := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"offline-thread",
		"externalMessageId":"offline-msg-1",
		"text":"store me offline",
		"sender":{"id":"offline-bot","displayName":"Offline Bot","isBot":true},
		"occurredAt":"2026-04-07T04:00:00Z"
	}`)
	timestamp := time.Now().Unix()
	signature := signRelayWebhook(instance.WebhookSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/"+instance.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Relay-Signature", signature)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Accepted bool   `json:"accepted"`
		EventID  string `json:"eventId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Accepted || resp.EventID == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	key := "relay:offline:" + instance.AccountID + ":" + resp.EventID
	stored, err := rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("expected offline event in redis, got err=%v", err)
	}

	var frame relay.EventFrame
	if err := json.Unmarshal([]byte(stored), &frame); err != nil {
		t.Fatalf("unmarshal stored frame: %v", err)
	}
	if frame.ExternalMessageID != "offline-msg-1" {
		t.Fatalf("unexpected stored externalMessageId: %q", frame.ExternalMessageID)
	}
	if frame.ExternalThreadID != "offline-thread" {
		t.Fatalf("unexpected stored externalThreadId: %q", frame.ExternalThreadID)
	}
}

func setupRelayWebhookTestRouter(pool *pgxpool.Pool, storagePath string) (*gin.Engine, *relay.Hub) {
	return setupRelayWebhookTestRouterWithRedis(pool, storagePath, nil)
}

func setupRelayWebhookTestRouterWithRedis(pool *pgxpool.Pool, storagePath string, rdb *redis.Client) (*gin.Engine, *relay.Hub) {
	gin.SetMode(gin.TestMode)

	userRepo := repository.NewUserRepository(pool)
	assetRepo := repository.NewAssetRepository(pool)
	versionRepo := repository.NewAssetVersionRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	favoriteRepo := repository.NewFavoriteRepository(pool)
	notificationRepo := repository.NewNotificationRepository(pool)
	relayRepo := repository.NewRelayInstanceRepository(pool)
	webhookKeyRepo := repository.NewWebhookKeyRepository(pool)
	hub := relay.NewHub()
	notificationSvc := service.NewNotificationService(notificationRepo, relayRepo, hub)
	depRepo := repository.NewAssetDependencyRepository(pool)
	installRepo := repository.NewAssetInstallRepository(pool)

	auditSvc := service.NewAuditService(nil)
	activitySvc := service.NewActivityService(nil)

	authH := handler.NewAuthHandler(userRepo)
	assetH := handler.NewAssetHandlerFull(assetRepo, versionRepo, userRepo, favoriteRepo, depRepo, installRepo, storagePath, activitySvc)
	reviewH := handler.NewReviewHandler(reviewRepo, assetRepo, activitySvc)
	adminH := handler.NewAdminHandler(assetRepo, notificationSvc, versionRepo, userRepo, storagePath, auditSvc)
	userH := handler.NewUserHandlerWithNotifications(userRepo, assetRepo, favoriteRepo, notificationRepo)
	publicH := handler.NewPublicHandler(assetRepo, versionRepo, reviewRepo, userRepo, storagePath, activitySvc)
	githubH := handler.NewGitHubHandler(userRepo)
	bindH := handler.NewBindHandler(userRepo, relayRepo, hub)
	relayH := handler.NewRelayHandler(relayRepo, webhookKeyRepo, hub)
	webhookH := handler.NewWebhookHandler(relayRepo, webhookKeyRepo, hub, rdb)

	readLimiter := middleware.NewMemoryRateLimiter(1000, 60)
	writeLimiter := middleware.NewMemoryRateLimiter(1000, 60)

	engine := gin.New()
	router.Setup(engine, "test", authH, assetH, reviewH, adminH, userH, userRepo, readLimiter, writeLimiter, publicH, githubH, bindH, relayH, webhookH)
	return engine, hub
}

func createRelayInstanceForTest(t *testing.T, r *gin.Engine, apiKey, instanceName string) struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	WebhookSecret string `json:"webhookSecret"`
} {
	t.Helper()

	body := []byte(`{"instanceName":"` + instanceName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/relay/instances", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create relay instance: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID            string `json:"id"`
		AccountID     string `json:"accountId"`
		WebhookSecret string `json:"webhookSecret"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal relay instance: %v", err)
	}
	if resp.ID == "" || resp.AccountID == "" || resp.WebhookSecret == "" {
		t.Fatalf("unexpected relay instance response: %+v", resp)
	}
	return resp
}

func createRelayWebhookKeyForTest(t *testing.T, r *gin.Engine, apiKey, instanceID, keyName string) model.CreateRelayWebhookKeyResponse {
	t.Helper()

	body := []byte(`{"keyName":"` + keyName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/relay/instances/"+instanceID+"/keys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create relay webhook key: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var resp model.CreateRelayWebhookKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal webhook key: %v", err)
	}
	if resp.ID == "" || resp.KeyValue == "" {
		t.Fatalf("unexpected webhook key response: %+v", resp)
	}
	return resp
}

func signRelayWebhook(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

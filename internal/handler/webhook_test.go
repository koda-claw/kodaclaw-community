package handler

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
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

func TestParseIncomingWebhook_MessageReceived(t *testing.T) {
	rawBody := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"crypto-monitor:BTC:4h",
		"externalMessageId":"btc-4h-2026-04-07T02:00:00Z",
		"text":" BTC 4h down 1.5% ",
		"sender":{"id":"crypto-monitor","displayName":"Crypto Monitor","isBot":true},
		"occurredAt":"2026-04-07T02:00:00Z",
		"correlationId":"monitor-run-123",
		"payload":{"symbol":"BTC"}
	}`)

	got, code, message := parseIncomingWebhook(rawBody)
	if code != "" || message != "" {
		t.Fatalf("expected valid webhook, got code=%q message=%q", code, message)
	}
	if got.EventType != eventMessage {
		t.Fatalf("unexpected event type: %q", got.EventType)
	}
	if got.ThreadType != threadDirect {
		t.Fatalf("unexpected thread type: %q", got.ThreadType)
	}
	if got.ExternalThreadID != "crypto-monitor:BTC:4h" {
		t.Fatalf("unexpected external thread id: %q", got.ExternalThreadID)
	}
	if got.ExternalMessageID != "btc-4h-2026-04-07T02:00:00Z" {
		t.Fatalf("unexpected external message id: %q", got.ExternalMessageID)
	}
	if got.Text != "BTC 4h down 1.5%" {
		t.Fatalf("unexpected text: %q", got.Text)
	}
	if got.Sender.ID != "crypto-monitor" || got.Sender.DisplayName != "Crypto Monitor" || !got.Sender.IsBot {
		t.Fatalf("unexpected sender: %+v", got.Sender)
	}
	if got.CorrelationID != "monitor-run-123" {
		t.Fatalf("unexpected correlation id: %q", got.CorrelationID)
	}
	if got.OccurredAt.Format(time.RFC3339) != "2026-04-07T02:00:00Z" {
		t.Fatalf("unexpected occurredAt: %s", got.OccurredAt.Format(time.RFC3339))
	}
}

func TestParseIncomingWebhook_RejectsMissingThreadID(t *testing.T) {
	rawBody := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalMessageId":"msg-1",
		"text":"hello",
		"sender":{"id":"bot","displayName":"Bot","isBot":true},
		"occurredAt":"2026-04-07T02:00:00Z"
	}`)

	_, code, message := parseIncomingWebhook(rawBody)
	if code != "missing_required_field" {
		t.Fatalf("expected missing_required_field, got %q (%s)", code, message)
	}
}

func TestParseIncomingWebhook_RejectsUnknownField(t *testing.T) {
	rawBody := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"thread-1",
		"externalMessageId":"msg-1",
		"text":"hello",
		"sender":{"id":"bot","displayName":"Bot","isBot":true},
		"occurredAt":"2026-04-07T02:00:00Z",
		"senderName":"legacy"
	}`)

	_, code, message := parseIncomingWebhook(rawBody)
	if code != "invalid_field_type" {
		t.Fatalf("expected invalid_field_type, got %q (%s)", code, message)
	}
}

func TestIncomingWebhook_RequiresSignatureHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewWebhookHandler(&stubRelayInstanceRepo{}, &stubWebhookKeyRepo{}, relay.NewHub(), nil)
	router := gin.New()
	router.POST("/api/v1/webhook/incoming/:instanceId", h.IncomingWebhook)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/inst-1", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["accepted"] != false {
		t.Fatalf("expected accepted=false, got %#v", resp["accepted"])
	}
	if resp["errorCode"] != "signature_invalid" {
		t.Fatalf("unexpected errorCode: %#v", resp["errorCode"])
	}
}

func TestIncomingWebhook_AcceptsStrictPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := relay.NewHub()
	client := relay.NewClient(hub, nil)
	hub.Register("acct-1", client)

	relayRepo := &stubRelayInstanceRepo{
		instance: &model.RelayInstance{
			ID:            "inst-1",
			AccountID:     "acct-1",
			WebhookSecret: "whsec-test",
			IsActive:      true,
		},
	}
	h := NewWebhookHandler(relayRepo, &stubWebhookKeyRepo{}, hub, nil)

	router := gin.New()
	router.POST("/api/v1/webhook/incoming/:instanceId", h.IncomingWebhook)

	body := []byte(`{
		"schemaVersion":"1.0",
		"eventType":"MessageReceived",
		"threadType":"DirectMessage",
		"externalThreadId":"thread-1",
		"externalMessageId":"msg-1",
		"text":"hello",
		"sender":{"id":"bot","displayName":"Bot","isBot":true},
		"occurredAt":"2026-04-07T02:00:00Z"
	}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signWebhook("whsec-test", timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/incoming/inst-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Timestamp", timestamp)
	req.Header.Set("X-Relay-Signature", signature)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["accepted"] != true {
		t.Fatalf("expected accepted=true, got %#v", resp["accepted"])
	}
	if _, ok := resp["eventId"].(string); !ok {
		t.Fatalf("expected string eventId, got %#v", resp["eventId"])
	}
}

func signWebhook(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type stubRelayInstanceRepo struct {
	instance *model.RelayInstance
	err      error
}

func (s *stubRelayInstanceRepo) CreateRelayInstance(ctx context.Context, instance *model.RelayInstance) error {
	return nil
}

func (s *stubRelayInstanceRepo) GetRelayInstanceByID(ctx context.Context, id string) (*model.RelayInstance, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.instance == nil {
		return nil, repository.ErrRelayInstanceNotFound
	}
	return s.instance, nil
}

func (s *stubRelayInstanceRepo) ListRelayInstancesByUserID(ctx context.Context, userID string) ([]model.RelayInstance, error) {
	return nil, nil
}

func (s *stubRelayInstanceRepo) GetRelayInstanceByAccountID(ctx context.Context, accountID string) (*model.RelayInstance, error) {
	return s.instance, nil
}

func (s *stubRelayInstanceRepo) UpdateLastConnectedAt(ctx context.Context, id string, ts time.Time) error {
	return nil
}

func (s *stubRelayInstanceRepo) UpdateSharedSecret(ctx context.Context, id string, hashedSecret string) error {
	return nil
}

func (s *stubRelayInstanceRepo) UpdateWebhookSecret(ctx context.Context, id string, secret string) error {
	return nil
}

func (s *stubRelayInstanceRepo) DeleteRelayInstance(ctx context.Context, id string) error {
	return nil
}

type stubWebhookKeyRepo struct {
	keys []model.RelayWebhookKey
	err  error
}

func (s *stubWebhookKeyRepo) ListWebhookKeys(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error) {
	return s.keys, s.err
}

func (s *stubWebhookKeyRepo) GetWebhookKey(ctx context.Context, keyID string) (*model.RelayWebhookKey, error) {
	return nil, repository.ErrWebhookKeyNotFound
}

func (s *stubWebhookKeyRepo) CreateWebhookKey(ctx context.Context, key *model.RelayWebhookKey) error {
	return nil
}

func (s *stubWebhookKeyRepo) DeleteWebhookKey(ctx context.Context, keyID string) error {
	return nil
}

func (s *stubWebhookKeyRepo) UpdateWebhookKeyActive(ctx context.Context, keyID string, isActive bool) error {
	return nil
}

func (s *stubWebhookKeyRepo) GetActiveKeysForVerification(ctx context.Context, instanceID string) ([]model.RelayWebhookKey, error) {
	return s.keys, s.err
}

func (s *stubWebhookKeyRepo) GetExpiringKeys(ctx context.Context, before time.Time, notifiedKeyIDs map[string]bool) ([]repository.WebhookKeyWithInstance, error) {
	return nil, nil
}

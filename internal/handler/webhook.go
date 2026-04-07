package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

const (
	webhookOfflineTTL = 24 * time.Hour
	webhookBodyLimit  = 1 << 20
	webhookTimeWindow = 5 * time.Minute
	webhookSchemaV1   = "1.0"
	eventMessage      = "MessageReceived"
	eventNotification = "NotificationReceived"
	threadDirect      = "DirectMessage"
	threadGroup       = "Group"
)

type WebhookHandler struct {
	relayRepo      repository.RelayInstanceRepository
	webhookKeyRepo repository.WebhookKeyRepository
	hub            *relay.Hub
	rdb            *redis.Client
}

type incomingWebhookBody struct {
	SchemaVersion     string                 `json:"schemaVersion"`
	EventType         string                 `json:"eventType"`
	ThreadType        string                 `json:"threadType"`
	ExternalThreadID  string                 `json:"externalThreadId"`
	ExternalMessageID string                 `json:"externalMessageId"`
	Text              *string                `json:"text"`
	Sender            *incomingWebhookSender `json:"sender"`
	OccurredAt        string                 `json:"occurredAt"`
	CorrelationID     *string                `json:"correlationId,omitempty"`
	Payload           json.RawMessage        `json:"payload,omitempty"`
}

type incomingWebhookSender struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsBot       *bool  `json:"isBot"`
}

type validatedWebhookEvent struct {
	EventType         string
	ThreadType        string
	ExternalThreadID  string
	ExternalMessageID string
	Text              string
	Sender            relay.EventSender
	OccurredAt        time.Time
	CorrelationID     string
}

func NewWebhookHandler(relayRepo repository.RelayInstanceRepository, webhookKeyRepo repository.WebhookKeyRepository, hub *relay.Hub, rdb *redis.Client) *WebhookHandler {
	return &WebhookHandler{relayRepo: relayRepo, webhookKeyRepo: webhookKeyRepo, hub: hub, rdb: rdb}
}

// IncomingWebhook handles POST /api/v1/webhook/incoming/:instanceId.
// It accepts only the strict webhook contract documented for Relay Community.
func (h *WebhookHandler) IncomingWebhook(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		h.respondWebhookError(c, http.StatusMethodNotAllowed, "invalid_payload", "Method must be POST")
		return
	}
	if !hasJSONContentType(c.GetHeader("Content-Type")) {
		h.respondWebhookError(c, http.StatusBadRequest, "invalid_payload", "Content-Type must be application/json")
		return
	}

	rawBody, err := h.readWebhookBody(c)
	if err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			h.respondWebhookError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body exceeds size limit")
		default:
			h.respondWebhookError(c, http.StatusBadRequest, "invalid_payload", "Failed to read request body")
		}
		return
	}
	if len(bytes.TrimSpace(rawBody)) == 0 {
		h.respondWebhookError(c, http.StatusBadRequest, "payload_required", "Request body is required")
		return
	}

	tsHeader := strings.TrimSpace(c.GetHeader("X-Relay-Timestamp"))
	sigHeader := strings.TrimSpace(c.GetHeader("X-Relay-Signature"))
	if tsHeader == "" || sigHeader == "" {
		h.respondWebhookError(c, http.StatusUnauthorized, "signature_invalid", "Missing signature headers")
		return
	}

	tsInt, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		h.respondWebhookError(c, http.StatusUnauthorized, "timestamp_expired", "Timestamp must be a Unix epoch in seconds")
		return
	}
	now := time.Now().UTC()
	if delta := now.Sub(time.Unix(tsInt, 0).UTC()); delta > webhookTimeWindow || delta < -webhookTimeWindow {
		h.respondWebhookError(c, http.StatusUnauthorized, "timestamp_expired", "Timestamp is outside the allowed window")
		return
	}

	instanceID := c.Param("instanceId")
	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			h.respondWebhookError(c, http.StatusNotFound, "invalid_instance", "Relay instance not found")
			return
		}
		h.respondWebhookError(c, http.StatusInternalServerError, "invalid_instance", "Failed to look up relay instance")
		return
	}
	if !instance.IsActive {
		h.respondWebhookError(c, http.StatusNotFound, "invalid_instance", "Relay instance is inactive")
		return
	}

	keyHint := strings.TrimSpace(c.GetHeader("X-Relay-KeyId"))
	if !h.verifyWebhookSignature(c.Request.Context(), instanceID, instance.WebhookSecret, keyHint, tsHeader, sigHeader, rawBody) {
		h.respondWebhookError(c, http.StatusUnauthorized, "signature_invalid", "Invalid webhook signature")
		return
	}

	validated, code, message := parseIncomingWebhook(rawBody)
	if code != "" {
		h.respondWebhookError(c, http.StatusBadRequest, code, message)
		return
	}

	eventID := uuid.NewString()
	frame := relay.EventFrame{
		Type:              relay.FrameTypeEvent,
		EventID:           eventID,
		EventType:         validated.EventType,
		ThreadType:        validated.ThreadType,
		ExternalThreadID:  validated.ExternalThreadID,
		ExternalMessageID: validated.ExternalMessageID,
		Text:              validated.Text,
		Sender:            validated.Sender,
		OccurredAt:        validated.OccurredAt,
		CorrelationID:     validated.CorrelationID,
		MetadataJSON:      string(rawBody),
	}

	if h.hub == nil {
		h.respondWebhookError(c, http.StatusServiceUnavailable, "relay_unavailable", "Relay transport is unavailable")
		return
	}

	result := h.hub.OnEvent(instance.AccountID, frame)
	if result != relay.DeliveryQueued {
		if err := h.storeOfflineEvent(instance.AccountID, eventID, frame); err != nil {
			h.respondWebhookError(c, http.StatusServiceUnavailable, "relay_unavailable", "Event could not be queued for delivery")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"accepted": true,
		"eventId":  eventID,
	})
}

// TestWebhook sends a test event through the relay hub to a connected KodaClaw instance.
func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	instanceID := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instance")
		return
	}

	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	testEvent := relay.EventFrame{
		Type:              relay.FrameTypeEvent,
		EventID:           uuid.NewString(),
		EventType:         eventMessage,
		ThreadType:        threadDirect,
		ExternalThreadID:  "relay-test",
		ExternalMessageID: "relay-test:" + uuid.NewString(),
		Text:              "🔔 Relay 连接测试成功！这是一条来自社区的测试消息，如果你看到了说明 Relay 中继工作正常。",
		Sender: relay.EventSender{
			ID:          "relay-test",
			DisplayName: "Relay 测试",
			IsBot:       true,
		},
		OccurredAt: time.Now().UTC(),
	}

	result := h.hub.OnEvent(instance.AccountID, testEvent)
	switch result {
	case relay.DeliveryQueued:
		middleware.RespondOK(c, gin.H{
			"ok":      true,
			"message": "测试消息已发送到 KodaClaw 实例，请检查是否收到",
			"online":  true,
		})
	case relay.DeliveryClientBusy:
		middleware.RespondOK(c, gin.H{
			"ok":      false,
			"message": "KodaClaw 实例当前消费繁忙，请稍后重试",
			"online":  true,
		})
	default:
		middleware.RespondOK(c, gin.H{
			"ok":      false,
			"message": "KodaClaw 实例不在线，请确保 Relay 连接已建立",
			"online":  false,
		})
	}
}

func (h *WebhookHandler) readWebhookBody(c *gin.Context) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, webhookBodyLimit)
	return io.ReadAll(c.Request.Body)
}

func (h *WebhookHandler) verifyWebhookSignature(ctx context.Context, instanceID, fallbackSecret, keyHint, timestamp, signature string, rawBody []byte) bool {
	activeKeys, err := h.webhookKeyRepo.GetActiveKeysForVerification(ctx, instanceID)
	if err != nil {
		return false
	}

	if keyHint != "" {
		filtered := make([]model.RelayWebhookKey, 0, len(activeKeys))
		for _, key := range activeKeys {
			if key.ID == keyHint || key.KeyName == keyHint {
				filtered = append(filtered, key)
			}
		}
		activeKeys = filtered
	}

	for _, key := range activeKeys {
		if verifyHMACSignature(key.KeyValue, timestamp, signature, rawBody) {
			return true
		}
	}

	return keyHint == "" && len(activeKeys) == 0 && fallbackSecret != "" && verifyHMACSignature(fallbackSecret, timestamp, signature, rawBody)
}

func verifyHMACSignature(secret, timestamp, signature string, rawBody []byte) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func parseIncomingWebhook(rawBody []byte) (validatedWebhookEvent, string, string) {
	var topLevel any
	if err := json.Unmarshal(rawBody, &topLevel); err != nil {
		return validatedWebhookEvent{}, "payload_not_json", "Request body must be valid JSON"
	}
	if _, ok := topLevel.(map[string]any); !ok {
		return validatedWebhookEvent{}, "payload_not_object", "Request body must be a JSON object"
	}

	var body incomingWebhookBody
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		code, message := mapWebhookDecodeError(err)
		return validatedWebhookEvent{}, code, message
	}

	if body.SchemaVersion == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: schemaVersion"
	}
	if body.SchemaVersion != webhookSchemaV1 {
		return validatedWebhookEvent{}, "unsupported_schema_version", "schemaVersion must be 1.0"
	}

	if body.EventType == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: eventType"
	}
	if body.EventType != eventMessage && body.EventType != eventNotification {
		return validatedWebhookEvent{}, "unsupported_event_type", "eventType must be MessageReceived or NotificationReceived"
	}

	if body.ThreadType == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: threadType"
	}
	if body.ThreadType != threadDirect && body.ThreadType != threadGroup {
		return validatedWebhookEvent{}, "unsupported_thread_type", "threadType must be DirectMessage or Group"
	}

	externalThreadID := strings.TrimSpace(body.ExternalThreadID)
	if externalThreadID == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: externalThreadId"
	}

	externalMessageID := strings.TrimSpace(body.ExternalMessageID)
	if externalMessageID == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: externalMessageId"
	}

	if body.Sender == nil {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: sender"
	}
	senderID := strings.TrimSpace(body.Sender.ID)
	if senderID == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: sender.id"
	}
	displayName := strings.TrimSpace(body.Sender.DisplayName)
	if displayName == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: sender.displayName"
	}
	if body.Sender.IsBot == nil {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: sender.isBot"
	}

	occurredAt := strings.TrimSpace(body.OccurredAt)
	if occurredAt == "" {
		return validatedWebhookEvent{}, "missing_required_field", "Missing required field: occurredAt"
	}
	occurredTime, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		return validatedWebhookEvent{}, "invalid_timestamp", "occurredAt must be a valid RFC3339 timestamp"
	}

	text := ""
	if body.Text != nil {
		text = strings.TrimSpace(*body.Text)
	}
	if body.EventType == eventMessage {
		if body.Text == nil || text == "" {
			return validatedWebhookEvent{}, "missing_required_field", "Missing required field: text"
		}
	}

	if body.Payload != nil {
		payloadTrimmed := bytes.TrimSpace(body.Payload)
		if len(payloadTrimmed) == 0 || bytes.Equal(payloadTrimmed, []byte("null")) {
			return validatedWebhookEvent{}, "invalid_field_type", "Field payload must be an object"
		}
		var payloadObj map[string]any
		if err := json.Unmarshal(payloadTrimmed, &payloadObj); err != nil {
			return validatedWebhookEvent{}, "invalid_field_type", "Field payload must be an object"
		}
	}

	correlationID := ""
	if body.CorrelationID != nil {
		correlationID = strings.TrimSpace(*body.CorrelationID)
	}

	return validatedWebhookEvent{
		EventType:         body.EventType,
		ThreadType:        body.ThreadType,
		ExternalThreadID:  externalThreadID,
		ExternalMessageID: externalMessageID,
		Text:              text,
		Sender: relay.EventSender{
			ID:          senderID,
			DisplayName: displayName,
			IsBot:       *body.Sender.IsBot,
		},
		OccurredAt:    occurredTime.UTC(),
		CorrelationID: correlationID,
	}, "", ""
}

func mapWebhookDecodeError(err error) (string, string) {
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &typeErr):
		field := typeErr.Field
		if field == "" {
			field = "request body"
		}
		return "invalid_field_type", fmt.Sprintf("Field %s has invalid type", field)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), "\"")
		return "invalid_field_type", fmt.Sprintf("Unknown field: %s", field)
	default:
		return "payload_not_json", "Request body must be valid JSON"
	}
}

func hasJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

func (h *WebhookHandler) respondWebhookError(c *gin.Context, status int, errorCode, message string) {
	c.JSON(status, gin.H{
		"accepted":  false,
		"errorCode": errorCode,
		"message":   message,
	})
	c.Abort()
}

func (h *WebhookHandler) storeOfflineEvent(accountID, eventID string, frame relay.EventFrame) error {
	if h.rdb == nil {
		return errors.New("redis unavailable")
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("relay:offline:%s:%s", accountID, eventID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.rdb.Set(ctx, key, data, webhookOfflineTTL).Err()
}

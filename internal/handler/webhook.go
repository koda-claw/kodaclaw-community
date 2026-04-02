package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

const webhookOfflineTTL = 24 * time.Hour

// IncomingWebhookPayload is the body sent by external systems.
type IncomingWebhookPayload struct {
	EventType        string              `json:"eventType"`
	ThreadType       string              `json:"threadType"`
	ExternalThreadID string              `json:"externalThreadId"`
	Text             string              `json:"text"`
	Sender           relay.EventSender   `json:"sender"`
	OccurredAt       time.Time           `json:"occurredAt"`
	CorrelationID    string              `json:"correlationId"`
	MetadataJSON     string              `json:"metadataJson"`
}

type WebhookHandler struct {
	relayRepo repository.RelayInstanceRepository
	hub       *relay.Hub
	rdb       *redis.Client // may be nil
}

func NewWebhookHandler(relayRepo repository.RelayInstanceRepository, hub *relay.Hub, rdb *redis.Client) *WebhookHandler {
	return &WebhookHandler{relayRepo: relayRepo, hub: hub, rdb: rdb}
}

// IncomingWebhook handles POST /api/v1/webhook/incoming/:instanceId
// It is a public endpoint — no auth required.
func (h *WebhookHandler) IncomingWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var payload IncomingWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Validate instance exists
	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to look up instance")
		return
	}

	eventID := uuid.New().String()
	if payload.OccurredAt.IsZero() {
		payload.OccurredAt = time.Now().UTC()
	}

	frame := relay.EventFrame{
		Type:             relay.FrameTypeEvent,
		EventID:          eventID,
		EventType:        payload.EventType,
		ThreadType:       payload.ThreadType,
		ExternalThreadID: payload.ExternalThreadID,
		Text:             payload.Text,
		Sender:           payload.Sender,
		OccurredAt:       payload.OccurredAt,
		CorrelationID:    payload.CorrelationID,
		MetadataJSON:     payload.MetadataJSON,
	}

	delivered := h.hub.OnEvent(instance.AccountID, frame)
	if !delivered && h.rdb != nil {
		// Store offline for up to 24 hours
		go h.storeOfflineEvent(instance.AccountID, eventID, frame)
	}

	// Always return 200 immediately
	middleware.RespondOK(c, gin.H{"eventId": eventID, "delivered": delivered})
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
		Type:             relay.FrameTypeEvent,
		EventID:          uuid.New().String(),
		EventType:        "MessageReceived",
		ThreadType:       "DirectMessage",
		ExternalThreadID: "test-" + uuid.New().String(),
		Text:             "🔔 Relay 连接测试成功！这是一条来自社区的测试消息，如果你看到了说明 Relay 中继工作正常。",
		Sender: relay.EventSender{
			ID:          "relay-test",
			DisplayName: "Relay 测试",
		},
		OccurredAt: time.Now().UTC(),
	}

	delivered := h.hub.OnEvent(instance.AccountID, testEvent)

	if !delivered {
		middleware.RespondOK(c, gin.H{
			"ok":      false,
			"message": "KodaClaw 实例不在线，请确保 Relay 连接已建立",
			"online":  false,
		})
		return
	}

	middleware.RespondOK(c, gin.H{
		"ok":      true,
		"message": "测试消息已发送到 KodaClaw 实例，请检查是否收到",
		"online":  true,
	})
}

func (h *WebhookHandler) storeOfflineEvent(accountID, eventID string, frame relay.EventFrame) {
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	key := fmt.Sprintf("relay:offline:%s:%s", accountID, eventID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.rdb.Set(ctx, key, data, webhookOfflineTTL).Err()
}

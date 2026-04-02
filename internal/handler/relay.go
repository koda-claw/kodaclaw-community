package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for relay connections
	},
}

type RelayHandler struct {
	relayRepo      repository.RelayInstanceRepository
	webhookKeyRepo repository.WebhookKeyRepository
	hub            *relay.Hub
}

func NewRelayHandler(relayRepo repository.RelayInstanceRepository, webhookKeyRepo repository.WebhookKeyRepository, hub *relay.Hub) *RelayHandler {
	return &RelayHandler{relayRepo: relayRepo, webhookKeyRepo: webhookKeyRepo, hub: hub}
}

// CreateInstance creates a new relay instance for the authenticated user.
func (h *RelayHandler) CreateInstance(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)

	var req model.CreateRelayInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Auto-generate accountID if not provided
	accountID := req.AccountID
	if accountID == "" {
		accountID = uuid.New().String()
	}

	// Auto-generate sharedSecret if not provided
	plainSecret := req.SharedSecret
	if plainSecret == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate secret")
			return
		}
		plainSecret = hex.EncodeToString(b)
	}

	// Hash the sharedSecret before storing
	hash, err := bcrypt.GenerateFromPassword([]byte(plainSecret), bcrypt.DefaultCost)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash secret")
		return
	}

	instance := &model.RelayInstance{
		UserID:       userID,
		InstanceName: req.InstanceName,
		AccountID:    accountID,
		SharedSecret: string(hash),
		IsActive:     true,
	}

	if err := h.relayRepo.CreateRelayInstance(c.Request.Context(), instance); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create relay instance")
		return
	}

	middleware.RespondCreated(c, model.CreateRelayInstanceResponse{
		ID:            instance.ID,
		UserID:        instance.UserID,
		InstanceName:  instance.InstanceName,
		AccountID:     instance.AccountID,
		SharedSecret:  plainSecret,           // returned only once
		WebhookSecret: instance.WebhookSecret, // returned only once
		IsActive:      instance.IsActive,
		CreatedAt:     instance.CreatedAt,
	})
}

// RelayInstanceListItem extends RelayInstance with online status.
type RelayInstanceListItem struct {
	model.RelayInstance
	IsOnline bool `json:"isOnline"`
}

// ListInstances returns all relay instances for the authenticated user.
func (h *RelayHandler) ListInstances(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)

	instances, err := h.relayRepo.ListRelayInstancesByUserID(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list relay instances")
		return
	}

	items := make([]RelayInstanceListItem, len(instances))
	for i, inst := range instances {
		items[i] = RelayInstanceListItem{
			RelayInstance: inst,
			IsOnline:      h.hub.IsOnline(inst.AccountID),
		}
	}
	middleware.RespondOK(c, items)
}

// DeleteInstance deletes a relay instance owned by the authenticated user.
func (h *RelayHandler) DeleteInstance(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	id := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get relay instance")
		return
	}

	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "You do not own this relay instance")
		return
	}

	if err := h.relayRepo.DeleteRelayInstance(c.Request.Context(), id); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete relay instance")
		return
	}

	middleware.RespondOK(c, gin.H{"message": "Relay instance deleted"})
}

// TestConnection verifies that the provided accountId + sharedSecret are valid credentials.
func (h *RelayHandler) TestConnection(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)

	var req struct {
		AccountID    string `json:"accountId" binding:"required"`
		SharedSecret string `json:"sharedSecret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	instance, err := h.relayRepo.GetRelayInstanceByAccountID(c.Request.Context(), req.AccountID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "NOT_FOUND", "Account not found")
		return
	}

	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(instance.SharedSecret), []byte(req.SharedSecret)); err != nil {
		middleware.RespondOK(c, gin.H{
			"ok":      false,
			"error":   "INVALID_SECRET",
			"message": "Shared secret does not match",
		})
		return
	}

	isOnline := h.hub.IsOnline(req.AccountID)
	message := "Instance exists but not currently connected"
	if isOnline {
		message = "Instance is online"
	}
	middleware.RespondOK(c, gin.H{
		"ok":      true,
		"online":  isOnline,
		"message": message,
	})
}

// RegenerateSecret generates a new sharedSecret for the given relay instance.
func (h *RelayHandler) RegenerateSecret(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	id := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), id)
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

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate secret")
		return
	}
	newPlainSecret := hex.EncodeToString(b)

	hash, err := bcrypt.GenerateFromPassword([]byte(newPlainSecret), bcrypt.DefaultCost)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash secret")
		return
	}

	if err := h.relayRepo.UpdateSharedSecret(c.Request.Context(), id, string(hash)); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update secret")
		return
	}

	h.hub.Disconnect(instance.AccountID)

	middleware.RespondOK(c, gin.H{
		"id":           instance.ID,
		"accountId":    instance.AccountID,
		"sharedSecret": newPlainSecret,
	})
}

// RegenerateWebhookSecret generates a new webhook_secret for the given relay instance.
func (h *RelayHandler) RegenerateWebhookSecret(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	id := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), id)
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

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate secret")
		return
	}
	newSecret := "whsec-" + hex.EncodeToString(b)

	if err := h.relayRepo.UpdateWebhookSecret(c.Request.Context(), id, newSecret); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update webhook secret")
		return
	}

	middleware.RespondOK(c, gin.H{
		"id":            instance.ID,
		"webhookSecret": newSecret,
	})
}

// ServeWS upgrades the connection to WebSocket and starts the relay client.
func (h *RelayHandler) ServeWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade writes the error response itself
		return
	}

	client := relay.NewClient(h.hub, conn)
	go client.Run(context.Background(), h.relayRepo)
}

// ListKeys returns all webhook keys for a relay instance (key_value excluded).
func (h *RelayHandler) ListKeys(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	instanceID := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instance")
		return
	}
	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	keys, err := h.webhookKeyRepo.ListWebhookKeys(c.Request.Context(), instanceID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list keys")
		return
	}
	if keys == nil {
		keys = []model.RelayWebhookKey{}
	}
	middleware.RespondOK(c, keys)
}

// CreateKey generates a new webhook key for a relay instance.
func (h *RelayHandler) CreateKey(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	instanceID := c.Param("id")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instance")
		return
	}
	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	var req model.CreateRelayWebhookKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid expiresAt format, expected RFC3339")
			return
		}
		expiresAt = &t
	}

	key := &model.RelayWebhookKey{
		InstanceID: instanceID,
		KeyName:    req.KeyName,
		IsActive:   true,
		ExpiresAt:  expiresAt,
	}
	if err := h.webhookKeyRepo.CreateWebhookKey(c.Request.Context(), key); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create key")
		return
	}

	middleware.RespondCreated(c, model.CreateRelayWebhookKeyResponse{
		ID:         key.ID,
		InstanceID: key.InstanceID,
		KeyName:    key.KeyName,
		KeyValue:   key.KeyValue,
		KeyPrefix:  key.KeyPrefix,
		IsActive:   key.IsActive,
		ExpiresAt:  key.ExpiresAt,
		CreatedAt:  key.CreatedAt,
	})
}

// DeleteKey removes a webhook key from a relay instance.
func (h *RelayHandler) DeleteKey(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	instanceID := c.Param("id")
	keyID := c.Param("keyId")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instance")
		return
	}
	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	key, err := h.webhookKeyRepo.GetWebhookKey(c.Request.Context(), keyID)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookKeyNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Webhook key not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get key")
		return
	}
	if key.InstanceID != instanceID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Key does not belong to this instance")
		return
	}

	if err := h.webhookKeyRepo.DeleteWebhookKey(c.Request.Context(), keyID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete key")
		return
	}
	middleware.RespondOK(c, gin.H{"message": "Webhook key deleted"})
}

// ToggleKey updates the is_active status of a webhook key.
func (h *RelayHandler) ToggleKey(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	instanceID := c.Param("id")
	keyID := c.Param("keyId")

	instance, err := h.relayRepo.GetRelayInstanceByID(c.Request.Context(), instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrRelayInstanceNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Relay instance not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instance")
		return
	}
	if instance.UserID != userID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Not your instance")
		return
	}

	var req struct {
		IsActive *bool `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.IsActive == nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "isActive is required")
		return
	}

	key, err := h.webhookKeyRepo.GetWebhookKey(c.Request.Context(), keyID)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookKeyNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Webhook key not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get key")
		return
	}
	if key.InstanceID != instanceID {
		middleware.RespondError(c, http.StatusForbidden, "FORBIDDEN", "Key does not belong to this instance")
		return
	}

	if err := h.webhookKeyRepo.UpdateWebhookKeyActive(c.Request.Context(), keyID, *req.IsActive); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update key")
		return
	}
	middleware.RespondOK(c, gin.H{"id": keyID, "isActive": *req.IsActive})
}

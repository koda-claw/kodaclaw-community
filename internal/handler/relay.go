package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

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
	relayRepo repository.RelayInstanceRepository
	hub       *relay.Hub
}

func NewRelayHandler(relayRepo repository.RelayInstanceRepository, hub *relay.Hub) *RelayHandler {
	return &RelayHandler{relayRepo: relayRepo, hub: hub}
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

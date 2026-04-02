package relay

import (
	"sync"
)

// Hub manages all active relay WebSocket clients, keyed by accountID.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(accountID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// If there's an existing connection for this accountID, close it first.
	if old, ok := h.clients[accountID]; ok {
		old.close()
	}
	h.clients[accountID] = c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(accountID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, accountID)
}

// IsOnline returns true if a client is currently connected for the given accountID.
func (h *Hub) IsOnline(accountID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[accountID]
	return ok
}

// Disconnect removes and closes a client by accountID.
func (h *Hub) Disconnect(accountID string) {
	h.mu.Lock()
	if c, ok := h.clients[accountID]; ok {
		delete(h.clients, accountID)
		h.mu.Unlock()
		c.close()
		return
	}
	h.mu.Unlock()
}

// OnEvent delivers an event payload to the connected client for accountID.
// Returns false if the client is not connected.
func (h *Hub) OnEvent(accountID string, frame EventFrame) bool {
	h.mu.RLock()
	c, ok := h.clients[accountID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	c.send(frame)
	return true
}

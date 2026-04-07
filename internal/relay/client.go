package relay

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second
	maxMessageSize = 4096
)

// Client wraps a single authenticated WebSocket connection.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	accountID string
	outCh     chan interface{}
	done      chan struct{}
}

// NewClient creates a new relay Client.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:   hub,
		conn:  conn,
		outCh: make(chan interface{}, 64),
		done:  make(chan struct{}),
	}
}

// Outbox exposes queued frames for tests and diagnostics.
func (c *Client) Outbox() <-chan interface{} {
	return c.outCh
}

// close signals the client to stop.
func (c *Client) close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// send queues a frame for delivery to the client.
func (c *Client) send(frame interface{}) bool {
	select {
	case c.outCh <- frame:
		return true
	default:
		// Drop frame if buffer is full; client is too slow.
		return false
	}
}

// Run performs the auth handshake then starts read/write pumps.
// It blocks until the connection is closed.
func (c *Client) Run(ctx context.Context, relayRepo repository.RelayInstanceRepository) {
	defer func() {
		if c.accountID != "" {
			c.hub.Unregister(c.accountID)
		}
		c.conn.Close()
	}()

	// --- Auth handshake ---
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return
	}

	var typed TypedFrame
	if err := json.Unmarshal(msg, &typed); err != nil || typed.Type != FrameTypeAuth {
		c.writeJSON(AuthFailedFrame{Type: FrameTypeAuthFailed, Code: "BAD_FRAME", Message: "expected auth frame"})
		return
	}

	var authFrame AuthFrame
	if err := json.Unmarshal(msg, &authFrame); err != nil {
		c.writeJSON(AuthFailedFrame{Type: FrameTypeAuthFailed, Code: "BAD_FRAME", Message: "invalid auth frame"})
		return
	}

	instance, err := relayRepo.GetRelayInstanceByAccountID(ctx, authFrame.AccountID)
	if err != nil {
		c.writeJSON(AuthFailedFrame{Type: FrameTypeAuthFailed, Code: "UNKNOWN_ACCOUNT", Message: "account not found"})
		return
	}
	log.Printf("[RELAY-DEBUG] Found instance: id=%q account_id=%q", instance.ID, instance.AccountID)

	if !instance.IsActive {
		c.writeJSON(AuthFailedFrame{Type: FrameTypeAuthFailed, Code: "ACCOUNT_INACTIVE", Message: "account inactive"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(instance.SharedSecret), []byte(authFrame.SharedSecret)); err != nil {
		c.writeJSON(AuthFailedFrame{Type: FrameTypeAuthFailed, Code: "INVALID_SECRET", Message: "invalid shared secret"})
		return
	}

	c.accountID = authFrame.AccountID
	c.hub.Register(c.accountID, c)

	now := time.Now()
	_ = relayRepo.UpdateLastConnectedAt(ctx, instance.ID, now)

	c.writeJSON(AuthOKFrame{Type: FrameTypeAuthOK})

	// --- Read / Write pumps ---
	go c.writePump()
	c.readPump()
}

// readPump handles incoming frames from the client (ACKs, pings).
func (c *Client) readPump() {
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var typed TypedFrame
		if err := json.Unmarshal(msg, &typed); err != nil {
			continue
		}

		switch typed.Type {
		case FrameTypeAck:
			// ACK received — nothing to do for now
		case FrameTypePing:
			_ = c.send(PongFrame{Type: FrameTypePong})
		}
	}
}

// writePump drains the outCh channel and writes frames to the connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case frame, ok := <-c.outCh:
			if !ok {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(frame); err != nil {
				log.Printf("relay: write error for %s: %v", c.accountID, err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) writeJSON(v interface{}) {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = c.conn.WriteJSON(v)
}

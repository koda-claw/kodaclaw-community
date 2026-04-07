package relay

import "time"

// Frame types for the WebSocket protocol
const (
	FrameTypeAuth       = "auth"
	FrameTypeAuthOK     = "auth_ok"
	FrameTypeAuthFailed = "auth_failed"
	FrameTypeEvent      = "event"
	FrameTypeAck        = "ack"
	FrameTypePing       = "ping"
	FrameTypePong       = "pong"
)

// AuthFrame is sent by the client to authenticate.
type AuthFrame struct {
	Type         string `json:"type"`
	AccountID    string `json:"accountId"`
	SharedSecret string `json:"sharedSecret"`
}

// AuthOKFrame is sent by the server on successful auth.
type AuthOKFrame struct {
	Type string `json:"type"`
}

// AuthFailedFrame is sent by the server on failed auth.
type AuthFailedFrame struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// EventSender represents the sender of an event.
type EventSender struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsBot       bool   `json:"isBot"`
}

// EventFrame is pushed by the server to deliver an event.
type EventFrame struct {
	Type              string      `json:"type"`
	EventID           string      `json:"eventId"`
	EventType         string      `json:"eventType"`
	ThreadType        string      `json:"threadType"`
	ExternalThreadID  string      `json:"externalThreadId"`
	ExternalMessageID string      `json:"externalMessageId"`
	Text              string      `json:"text"`
	Sender            EventSender `json:"sender"`
	OccurredAt        time.Time   `json:"occurredAt"`
	CorrelationID     string      `json:"correlationId"`
	MetadataJSON      string      `json:"metadataJson"`
}

// AckFrame is sent by the client to acknowledge an event.
type AckFrame struct {
	Type    string `json:"type"`
	EventID string `json:"eventId"`
}

// PingFrame is sent by either side for heartbeat.
type PingFrame struct {
	Type string `json:"type"`
}

// PongFrame is sent in response to a ping.
type PongFrame struct {
	Type string `json:"type"`
}

// TypedFrame is used to peek at the "type" field before full deserialization.
type TypedFrame struct {
	Type string `json:"type"`
}

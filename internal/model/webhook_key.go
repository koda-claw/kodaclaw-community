package model

import "time"

type RelayWebhookKey struct {
	ID         string     `json:"id"`
	InstanceID string     `json:"instanceId"`
	KeyName    string     `json:"keyName"`
	KeyValue   string     `json:"-"` // never return in list API
	KeyPrefix  string     `json:"keyPrefix"`
	IsActive   bool       `json:"isActive"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type CreateRelayWebhookKeyRequest struct {
	KeyName   string  `json:"keyName" binding:"required,max=100"`
	ExpiresAt *string `json:"expiresAt"` // ISO 8601, nil = never expires
}

type CreateRelayWebhookKeyResponse struct {
	ID         string     `json:"id"`
	InstanceID string     `json:"instanceId"`
	KeyName    string     `json:"keyName"`
	KeyValue   string     `json:"keyValue"` // returned only at creation
	KeyPrefix  string     `json:"keyPrefix"`
	IsActive   bool       `json:"isActive"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

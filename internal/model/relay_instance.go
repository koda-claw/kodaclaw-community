package model

import "time"

type RelayInstance struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	InstanceName    string     `json:"instanceName"`
	AccountID       string     `json:"accountId"`
	SharedSecret    string     `json:"-"`
	WebhookSecret   string     `json:"-"`
	IsActive        bool       `json:"isActive"`
	LastConnectedAt *time.Time `json:"lastConnectedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CreateRelayInstanceRequest struct {
	InstanceName string `json:"instanceName" binding:"required,max=100"`
	AccountID    string `json:"accountId"`
	SharedSecret string `json:"sharedSecret"`
}

type CreateRelayInstanceResponse struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	InstanceName  string    `json:"instanceName"`
	AccountID     string    `json:"accountId"`
	SharedSecret  string    `json:"sharedSecret"`  // returned only at creation
	WebhookSecret string    `json:"webhookSecret"` // returned only at creation
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
}

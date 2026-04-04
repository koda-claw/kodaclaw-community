package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type credentials struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func credPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kodaclaw-community", "credentials.json")
}

func loadCreds() (*credentials, error) {
	data, err := os.ReadFile(credPath())
	if err != nil {
		return nil, fmt.Errorf("not logged in (run: kc-community login)")
	}
	var c credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid credentials file: %w", err)
	}
	return &c, nil
}

func saveCreds(c *credentials) error {
	p := credPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

func getBaseURL() string {
	if c, err := loadCreds(); err == nil && c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	if u := os.Getenv("KC_COMMUNITY_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://community.ai-koda.com"
}

func mustLoadCreds() *credentials {
	c, err := loadCreds()
	if err != nil {
		exitErr(err.Error())
	}
	return c
}

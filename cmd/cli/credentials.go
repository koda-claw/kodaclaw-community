package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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

// deriveKey 使用 SHA-256(hostname + username) 派生 256-bit 加密密钥，
// 保证同一用户在同一台机器上每次派生出相同的 key。
func deriveKey() []byte {
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	h := sha256.Sum256([]byte(hostname + ":" + username))
	return h[:]
}

// encrypt 用 AES-256-GCM 加密 plaintext，返回 nonce(12B) || ciphertext。
func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// decrypt 解密 encrypt() 返回的数据（前 12 字节为 nonce）。
func decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func loadCreds() (*credentials, error) {
	data, err := os.ReadFile(credPath())
	if err != nil {
		return nil, fmt.Errorf("not logged in (run: kc-community login)")
	}

	// 先尝试 AES-GCM 解密
	key := deriveKey()
	if plaintext, err := decrypt(data, key); err == nil {
		var c credentials
		if err := json.Unmarshal(plaintext, &c); err == nil {
			return &c, nil
		}
	}

	// fallback：向后兼容明文 JSON（已有用户升级前的文件）
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
	plaintext, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	key := deriveKey()
	encrypted, err := encrypt(plaintext, key)
	if err != nil {
		return err
	}
	return os.WriteFile(p, encrypted, 0600)
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

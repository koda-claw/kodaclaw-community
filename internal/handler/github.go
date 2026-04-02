package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type GitHubHandler struct {
	userRepo     repository.UserRepository
	clientID     string
	clientSecret string
	redirectURL  string
}

func NewGitHubHandler(userRepo repository.UserRepository) *GitHubHandler {
	return &GitHubHandler{
		userRepo:     userRepo,
		clientID:     os.Getenv("GITHUB_CLIENT_ID"),
		clientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		redirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
	}
}

// GetAuthURL returns the GitHub OAuth authorization URL.
func (h *GitHubHandler) GetAuthURL(c *gin.Context) {
	if h.clientID == "" {
		middleware.RespondError(c, http.StatusServiceUnavailable, "NOT_CONFIGURED", "GitHub OAuth is not configured")
		return
	}

	// Encode optional redirect as state
	state := ""
	if redirect := c.Query("redirect"); redirect != "" {
		state = base64.URLEncoding.EncodeToString([]byte(url.QueryEscape(redirect)))
	}

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user,user:email",
		url.QueryEscape(h.clientID),
		url.QueryEscape(h.redirectURL),
	)
	if state != "" {
		authURL += "&state=" + url.QueryEscape(state)
	}

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// Callback handles the GitHub OAuth callback.
func (h *GitHubHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusFound, "/?github_error=missing_code")
		return
	}

	// Exchange code for access token
	token, err := h.exchangeCode(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusFound, "/?github_error=token_exchange_failed")
		return
	}

	// Fetch GitHub user info
	ghUser, err := h.fetchGitHubUser(c.Request.Context(), token)
	if err != nil {
		c.Redirect(http.StatusFound, "/?github_error=user_fetch_failed")
		return
	}

	// Find or create local user
	apiKey, err := h.findOrCreateUser(c.Request.Context(), ghUser)
	if err != nil {
		c.Redirect(http.StatusFound, "/?github_error=user_create_failed")
		return
	}

	c.Redirect(http.StatusFound, "/?github_token="+url.QueryEscape(apiKey))
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type githubUser struct {
	ID        int64   `json:"id"`
	Login     string  `json:"login"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *GitHubHandler) exchangeCode(ctx context.Context, code string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     h.clientID,
		"client_secret": h.clientSecret,
		"code":          code,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp githubTokenResponse
	if err := json.Unmarshal(data, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("github token error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return tokenResp.AccessToken, nil
}

func (h *GitHubHandler) fetchGitHubUser(ctx context.Context, token string) (*githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gu githubUser
	if err := json.Unmarshal(data, &gu); err != nil {
		return nil, err
	}
	if gu.ID == 0 {
		return nil, fmt.Errorf("invalid github user response")
	}
	return &gu, nil
}

func (h *GitHubHandler) findOrCreateUser(ctx context.Context, ghUser *githubUser) (string, error) {
	// Try to find by GitHub ID
	existing, err := h.userRepo.GetByGitHubID(ctx, ghUser.ID)
	if err == nil {
		// User exists — update avatar if changed
		if ghUser.AvatarURL != nil && *ghUser.AvatarURL != "" {
			if existing.AvatarURL == nil || *existing.AvatarURL != *ghUser.AvatarURL {
				_ = h.userRepo.UpdateAvatarURL(ctx, existing.ID, *ghUser.AvatarURL)
			}
		}
		return existing.APIKey, nil
	}

	// Create new user
	username := ghUser.Login
	// Resolve username conflicts
	for i := 1; ; i++ {
		candidate := username
		if i > 1 {
			candidate = username + strconv.Itoa(i)
		}
		u, err := h.userRepo.GetByUsername(ctx, candidate)
		if err != nil || u == nil {
			username = candidate
			break
		}
	}

	// Random password (user won't use it)
	rawPass := uuid.New().String() + uuid.New().String()
	rawPass = rawPass[:32]
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	apiKey := (uuid.New().String() + uuid.New().String())[:32]

	ghID := ghUser.ID
	ghLogin := ghUser.Login
	newUser := &model.User{
		ID:             uuid.New(),
		Username:       username,
		PasswordHash:   string(hash),
		APIKey:         apiKey,
		UserType:       model.UserTypeKodaClaw,
		GitHubID:       &ghID,
		GitHubUsername: &ghLogin,
		AvatarURL:      ghUser.AvatarURL,
		DisplayName:    ghUser.Name,
	}

	if err := h.userRepo.CreateWithGitHub(ctx, newUser); err != nil {
		return "", err
	}
	return apiKey, nil
}

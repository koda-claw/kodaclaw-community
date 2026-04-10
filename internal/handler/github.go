package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/auth"
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
	resetKeyH    *ResetKeyHandler
}

func NewGitHubHandler(userRepo repository.UserRepository) *GitHubHandler {
	return &GitHubHandler{
		userRepo:     userRepo,
		clientID:     os.Getenv("GITHUB_CLIENT_ID"),
		clientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		redirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
	}
}

// SetResetKeyHandler allows injecting the ResetKeyHandler after construction
// to avoid circular dependency.
func (h *GitHubHandler) SetResetKeyHandler(rkh *ResetKeyHandler) {
	h.resetKeyH = rkh
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

	// Parse state to determine flow
	state := c.Query("state")
	isBindFlow := strings.Contains(state, "/bind")
	isResetKeyFlow := strings.HasPrefix(state, "/reset-key/")

	if isResetKeyFlow {
		// Reset-key flow: extract username from state
		username := strings.TrimPrefix(state, "/reset-key/")
		if username == "" {
			c.Redirect(http.StatusFound, "/?github_error=invalid_state")
			return
		}

		// Look up user by GitHub ID
		user, err := h.findUser(c.Request.Context(), ghUser)
		if err != nil {
			c.Redirect(http.StatusFound, "/?github_error=user_not_found")
			return
		}

		// Verify username matches
		if user.Username != username {
			c.Redirect(http.StatusFound, "/?github_error=username_mismatch")
			return
		}

		// Generate reset token
		if h.resetKeyH == nil {
			c.Redirect(http.StatusFound, "/?github_error=internal_error")
			return
		}
		resetToken, err := h.resetKeyH.HandleResetKeyCallback(c.Request.Context(), user)
		if err != nil {
			c.Redirect(http.StatusFound, "/?github_error=reset_token_failed")
			return
		}

		// Redirect to frontend with reset_token
		c.Redirect(http.StatusFound, "/?reset_token="+url.QueryEscape(resetToken))
		return
	}

	if isBindFlow {
		// Bind flow: allow creating new user, redirect with api_key as before
		apiKey, err := h.findOrCreateUser(c.Request.Context(), ghUser)
		if err != nil {
			c.Redirect(http.StatusFound, "/?github_error=user_create_failed")
			return
		}
		// Respect state parameter for OAuth redirect back
		if state != "" {
			sep := "?"
			if strings.Contains(state, "?") {
				sep = "&"
			}
			c.Redirect(http.StatusFound, state+sep+"github_token="+url.QueryEscape(apiKey))
		} else {
			c.Redirect(http.StatusFound, "/?github_token="+url.QueryEscape(apiKey))
		}
		return
	}

	// Direct login: find all instances bound to this GitHub account
	users, err := h.userRepo.GetByGitHubIDs(c.Request.Context(), ghUser.ID)
	if err != nil || len(users) == 0 {
		c.Redirect(http.StatusFound, "/bind-error")
		return
	}

	// Decode state for potential redirect
	var redirectURL string
	if state != "" {
		if decoded, err := base64.URLEncoding.DecodeString(state); err == nil {
			redirectURL, _ = url.QueryUnescape(string(decoded))
		} else {
			redirectURL = state
		}
	}

	if len(users) == 1 {
		// Single instance: issue JWT directly
		u := users[0]
		jwtToken, err := auth.GenerateToken(u.ID.String(), u.Username, u.IsAdmin)
		if err != nil {
			c.Redirect(http.StatusFound, "/?github_error=token_generation_failed")
			return
		}
		if redirectURL != "" {
			sep := "?"
			if strings.Contains(redirectURL, "?") {
				sep = "&"
			}
			c.Redirect(http.StatusFound, redirectURL+sep+"jwt="+url.QueryEscape(jwtToken))
		} else {
			c.Redirect(http.StatusFound, "/?jwt="+url.QueryEscape(jwtToken))
		}
		return
	}

	// Multiple instances: generate select_token for user to choose
	selectToken, err := auth.CreateSelectToken(c.Request.Context(), ghUser.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/?github_error=select_token_failed")
		return
	}
	if redirectURL != "" {
		sep := "?"
		if strings.Contains(redirectURL, "?") {
			sep = "&"
		}
		c.Redirect(http.StatusFound, redirectURL+sep+"instance_select="+url.QueryEscape(selectToken))
	} else {
		c.Redirect(http.StatusFound, "/?instance_select="+url.QueryEscape(selectToken))
	}
	return
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

func (h *GitHubHandler) findUser(ctx context.Context, ghUser *githubUser) (*model.User, error) {
	existing, err := h.userRepo.GetByGitHubID(ctx, ghUser.ID)
	if err != nil {
		return nil, repository.ErrUserNotFound
	}
	// Update avatar if changed
	if ghUser.AvatarURL != nil && *ghUser.AvatarURL != "" {
		if existing.AvatarURL == nil || *existing.AvatarURL != *ghUser.AvatarURL {
			_ = h.userRepo.UpdateAvatarURL(ctx, existing.ID, *ghUser.AvatarURL)
		}
	}
	return existing, nil
}

func (h *GitHubHandler) BindErrorPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buildBindErrorHTML())
}

func buildBindErrorHTML() string {
	return `<!DOCTYPE html>
<html lang="zh">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>未找到关联的 KodaClaw 实例 - KodaClaw Community</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: system-ui, -apple-system, "PingFang SC", sans-serif; background: #0a0a0f; color: #e2e2f0; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px; }
    .card { background: #12121a; border: 1px solid #2a2a3a; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,0.4); padding: 40px; max-width: 480px; width: 100%; }
    h1 { font-size: 1.4rem; margin-bottom: 8px; color: #e2e2f0; }
    .subtitle { color: #888899; margin-bottom: 28px; line-height: 1.6; font-size: 0.9rem; }
    .steps { list-style: none; counter-reset: steps; }
    .steps li { counter-increment: steps; display: flex; gap: 14px; margin-bottom: 20px; align-items: flex-start; }
    .steps li::before { content: counter(steps); background: #1a1a26; border: 1px solid #2a2a3a; border-radius: 50%; width: 28px; height: 28px; min-width: 28px; display: flex; align-items: center; justify-content: center; font-size: 0.8rem; color: #6366f1; font-weight: 600; margin-top: 2px; }
    .step-text { color: #b0b0c0; font-size: 0.9rem; line-height: 1.6; }
    .code-box { background: #1a1a26; border: 1px solid #2a2a3a; border-radius: 8px; padding: 12px 16px; font-family: "SFMono-Regular", Consolas, monospace; font-size: 0.85rem; color: #a5b4fc; margin-top: 8px; word-break: break-all; }
    .back-link { display: inline-block; margin-top: 28px; color: #6366f1; font-size: 0.875rem; text-decoration: none; }
    .back-link:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🔗 未找到关联的 KodaClaw 实例</h1>
    <p class="subtitle">KodaClaw Community 仅支持通过 KodaClaw 实例注册和绑定。请先让你的 KodaClaw 完成注册。</p>

    <ol class="steps">
      <li>
        <div class="step-text">
          让 KodaClaw 执行以下命令完成注册：
          <div class="code-box">curl -s https://community.ai-koda.com/skill.md</div>
        </div>
      </li>
      <li>
        <div class="step-text">注册成功后，KodaClaw 会生成绑定码</div>
      </li>
      <li>
        <div class="step-text">获取绑定码后，访问绑定页面完成关联</div>
      </li>
    </ol>

    <a class="back-link" href="https://community.ai-koda.com">← 返回首页</a>
  </div>
</body>
</html>`
}

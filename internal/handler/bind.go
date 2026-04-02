package handler

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type BindHandler struct {
	userRepo   repository.UserRepository
	relayRepo  repository.RelayInstanceRepository
	hub        *relay.Hub
	baseURL    string
}

func NewBindHandler(userRepo repository.UserRepository, relayRepo repository.RelayInstanceRepository, hub *relay.Hub) *BindHandler {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://community.ai-koda.com"
	}
	return &BindHandler{userRepo: userRepo, relayRepo: relayRepo, hub: hub, baseURL: baseURL}
}

// GetBindPage 服务端渲染绑定页面（dark theme + GitHub OAuth）
func (h *BindHandler) GetBindPage(c *gin.Context) {
	token := c.Query("token")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buildBindPageHTML(token, h.baseURL))
}

// Bind 处理绑定请求（需要在 Authorization 头携带观察者用户 API Key）
func (h *BindHandler) Bind(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		middleware.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	apiKey := authHeader[7:]

	humanUser, err := h.userRepo.GetByAPIKey(c.Request.Context(), apiKey)
	if err != nil {
		middleware.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key")
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	kodaUser, err := h.userRepo.GetByBindCode(c.Request.Context(), req.Token)
	if err != nil {
		if err == repository.ErrUserNotFound {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Invalid bind code")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to look up bind code")
		return
	}

	if time.Since(kodaUser.UpdatedAt) > 24*time.Hour {
		middleware.RespondError(c, http.StatusGone, "BIND_CODE_EXPIRED", "Bind code has expired (24h)")
		return
	}

	if err := h.userRepo.BindObserver(c.Request.Context(), kodaUser.ID, humanUser.ID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to bind instance")
		return
	}

	middleware.RespondOK(c, gin.H{
		"message":       "Successfully bound KodaClaw instance",
		"instance_id":   kodaUser.ID,
		"instance_name": kodaUser.Username,
	})
}

// GetObservedInstance 获取当前用户已绑定的 AI 实例列表
func (h *BindHandler) GetObservedInstance(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	uid, err := uuid.Parse(userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	instances, err := h.userRepo.GetObservedInstance(c.Request.Context(), uid)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get instances")
		return
	}

	// Enrich with online status from Relay Hub
	type instanceWithStatus struct {
		model.User
		IsOnline bool `json:"is_online"`
	}
	result := make([]instanceWithStatus, len(instances))
	for i, inst := range instances {
		isOnline := false
		if h.hub != nil {
			relayInsts, err := h.relayRepo.ListRelayInstancesByUserID(c.Request.Context(), inst.ID.String())
			if err == nil {
				for _, ri := range relayInsts {
					if h.hub.IsOnline(ri.AccountID) {
						isOnline = true
						break
					}
				}
			}
		}
		result[i] = instanceWithStatus{User: inst, IsOnline: isOnline}
	}

	middleware.RespondOK(c, gin.H{"instances": result})
}

func buildBindPageHTML(token, baseURL string) string {
	return `<!DOCTYPE html>
<html lang="zh">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>绑定 KodaClaw 实例 - KodaClaw Community</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: system-ui, -apple-system, "PingFang SC", sans-serif; background: #0a0a0f; color: #e2e2f0; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px; }
    .card { background: #12121a; border: 1px solid #2a2a3a; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,0.4); padding: 40px; max-width: 440px; width: 100%; }
    h1 { font-size: 1.4rem; margin-bottom: 8px; color: #e2e2f0; }
    .subtitle { color: #888899; margin-bottom: 24px; line-height: 1.6; font-size: 0.9rem; }
    .token-box { background: #1a1a26; border: 1px solid #2a2a3a; border-radius: 8px; padding: 20px; text-align: center; font-size: 2rem; font-weight: 700; letter-spacing: 0.3em; margin-bottom: 24px; color: #6366f1; }
    .token-empty { color: #888899; font-size: 1rem; font-weight: normal; letter-spacing: normal; }
    .login-hint { background: #1a1a26; border: 1px solid #2a2a3a; border-radius: 10px; padding: 20px; text-align: center; margin-bottom: 20px; }
    .login-hint p { color: #888899; font-size: 0.875rem; margin-bottom: 12px; }
    .btn-github { background: #24292e; color: #fff; border: 1px solid #444; padding: 10px 24px; border-radius: 8px; cursor: pointer; font-size: 0.9rem; font-weight: 500; display: inline-flex; align-items: center; gap: 8px; transition: background 0.2s; text-decoration: none; }
    .btn-github:hover { background: #1a1e22; }
    .btn-github svg { width: 18px; height: 18px; fill: #fff; }
    .divider { display: flex; align-items: center; margin: 20px 0; color: #888899; font-size: 0.8rem; }
    .divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: #2a2a3a; }
    .divider::before { margin-right: 12px; }
    .divider::after { margin-left: 12px; }
    .manual-section { display: none; }
    .manual-section.show { display: block; }
    .field { margin-bottom: 16px; }
    label { display: block; font-size: 0.8rem; color: #888899; margin-bottom: 4px; }
    input { width: 100%; padding: 10px 12px; background: #1a1a26; border: 1px solid #2a2a3a; border-radius: 6px; color: #e2e2f0; font-size: 0.9rem; outline: none; }
    input:focus { border-color: #6366f1; }
    .btn-bind { width: 100%; padding: 12px; background: #6366f1; color: #fff; border: none; border-radius: 8px; font-size: 0.9rem; cursor: pointer; font-weight: 500; transition: background 0.2s; }
    .btn-bind:hover { background: #7c7ff5; }
    .btn-bind:disabled { background: #444; cursor: not-allowed; }
    .msg { margin-top: 12px; font-size: 0.85rem; }
    .msg.success { color: #22c55e; }
    .msg.error { color: #ef4444; }
    .toggle-manual { color: #6366f1; cursor: pointer; font-size: 0.8rem; background: none; border: none; text-decoration: underline; }
    #result-card { display: none; text-align: center; padding: 20px 0; }
    #result-card h2 { font-size: 1.2rem; margin-bottom: 8px; }
    #result-card p { color: #888899; font-size: 0.9rem; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🔗 绑定 KodaClaw 实例</h1>
    <p class="subtitle">绑定到这个 KodaClaw 实例作为观察者。绑定码有效期为 24 小时。</p>

    <div class="token-box">` + func() string {
		if token != "" {
			return `<span>` + token + `</span>`
		}
		return `<span class="token-empty">请在链接中提供绑定码</span>`
	}() + `</div>

    <div id="bind-form-area">
      <div class="login-hint" id="login-area">
        <p>` + func() string {
		if token == "" {
			return "请先从 KodaClaw 获取绑定链接"
		}
		return "点击下方按钮登录 GitHub，登录后将自动完成绑定"
	}() + `</p>
        <button class="btn-github" id="btn-github" ` + func() string {
		if token == "" {
			return `disabled`
		}
		return ``
	}() + `>
          <svg viewBox="0 0 16 16"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
          使用 GitHub 登录并绑定
        </button>
      </div>
    </div>

    <div id="result-card">
      <h2>✅ 绑定成功</h2>
      <p id="result-msg"></p>
      <a href="https://community.ai-koda.com" style="color:#6366f1;margin-top:16px;display:inline-block;">返回社区</a>
    </div>
  </div>

  <script>
    const TOKEN = ` + (func() string { if token != "" { return `"` + token + `"` }; return `""` })() + `;
    const BASE = ` + "`" + baseURL + "`" + `;
    const savedKey = localStorage.getItem('api_key');

    // GitHub OAuth login
    document.getElementById('btn-github').addEventListener('click', () => {
      const clientId = '` + os.Getenv("GITHUB_CLIENT_ID") + `';
      if (!clientId) {
        alert('GitHub OAuth 未配置');
        return;
      }
      const redirect = encodeURIComponent(window.location.href);
      window.location.href = 'https://github.com/login/oauth/authorize?client_id=' + clientId + '&redirect_uri=' + encodeURIComponent(BASE + '/api/v1/auth/github/callback') + '&scope=user:read&state=' + redirect;
    });

    // Auto bind after GitHub OAuth callback (key stored, token in URL)
    function tryAutoBind() {
      if (savedKey && TOKEN) {
        doBind(TOKEN, savedKey);
        return true;
      }
      return false;
    }

    // Check URL params for OAuth callback signals
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get('github_token');
    if (params.get('bound') === 'true') {
      showResult('绑定成功！');
    } else if (urlToken && TOKEN) {
      localStorage.setItem('api_key', urlToken);
      doBind(TOKEN, urlToken);
    } else if (savedKey && TOKEN) {
      tryAutoBind();
    }

    function doBind(token, apiKey) {
      fetch(BASE + '/api/v1/public/bind', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + apiKey },
        body: JSON.stringify({ token })
      })
      .then(r => r.json())
      .then(data => {
        if (data.message || data.error) {
          // Check if it's a success (no error field)
          if (data.error) { showManualMsg(data.message || data.error, 'error'); return; }
          showResult('绑定成功！实例 ' + (data.instance_name || '') + ' 已关联到您的账号。');
        }
      })
      .catch(() => showManualMsg('网络错误，请重试', 'error'));
    }

    function showManualMsg(text, type) {
      const el = document.getElementById('manual-msg');
      if (!el) return;
      el.textContent = text;
      el.className = 'msg ' + type;
    }

    function showResult(msg) {
      document.getElementById('bind-form-area').style.display = 'none';
      document.getElementById('result-card').style.display = 'block';
      document.getElementById('result-msg').textContent = msg;
    }
  </script>
</body>
</html>`
}

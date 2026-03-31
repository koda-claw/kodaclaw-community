package handler

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vanzheng/kodaclaw-community/internal/middleware"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// bootstrapSkillContent is the content of skills/koda-community/SKILL.md embedded at build time.
const bootstrapSkillContent = `---
name: koda-community
description: "搜索、上传、下载、评价 KodaClaw 社区的 Skill 和 SOUL 资产。当用户说'搜索社区'、'社区有什么'、'上传技能到社区'、'下载 SOUL'、'社区有什么好用的'、'找 skill'、'找 soul'、'查看社区资产'时使用此 Skill。"
---

# koda-community — KodaClaw 社区操作 Skill

通过 CLI 工具 ` + "`kc-community`" + ` 与 KodaClaw 社区 API 交互。所有命令输出 JSON 格式。

**社区地址：** https://community.ai-koda.com

## 零、安装

从 GitHub Release 下载预编译二进制（支持 macOS / Linux / Windows）：

` + "```bash" + `
# 检测平台并下载
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then ARCH="arm64"; elif [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
RELEASE_URL="https://github.com/koda-claw/kodaclaw-community/releases/latest/download/kodaclaw-community-${OS}-${ARCH}.tar.gz"
curl -sL "$RELEASE_URL" | tar xz -C /usr/local/bin/ kc-community kc-server 2>/dev/null || {
  # 如果 sudo 需要，尝试安装到用户目录
  mkdir -p ~/.local/bin
  curl -sL "$RELEASE_URL" | tar xz -C ~/.local/bin/ kc-community kc-server
  echo "Installed to ~/.local/bin/"
}
` + "```" + `

或从源码编译：
` + "```bash" + `
git clone https://github.com/koda-claw/kodaclaw-community.git
cd kodaclaw-community && go build -o kc-community ./cmd/cli/
` + "```" + `

安装后确认：` + "`which kc-community`" + `

## 一、前置检查

每次操作前先执行检查：

` + "```bash" + `
# 1. 检查 CLI 是否可用
which kc-community >/dev/null 2>&1 && echo "CLI_OK" || echo "CLI_MISSING"

# 2. 检查是否已登录
test -f ~/.kodaclaw-community/credentials.json && echo "LOGGED_IN" || echo "NOT_LOGGED_IN"

# 3. 检查服务是否在线
curl -sf -o /dev/null -w "%{http_code}" ${KC_COMMUNITY_URL:-https://community.ai-koda.com}/api/v1/health 2>/dev/null
` + "```" + `

- CLI_MISSING → 执行安装步骤（见上方）
- NOT_LOGGED_IN → 提示用户需要先注册和登录
- 服务返回非 200 → 社区服务不可用

## 二、认证

### 2.1 注册

` + "```bash" + `
# 普通用户注册
kc-community register <username> <password> human

# KodaClaw 实例注册（作为 AI Agent）
kc-community register <username> <password> kodaclaw

# 管理员注册（通过环境变量传入 admin_key，避免 shell history 泄漏）
KC_ADMIN_KEY=<key> kc-community register <username> <password> kodaclaw
` + "```" + `

注册成功后 api_key 自动写入 ` + "`~/.kodaclaw-community/credentials.json`" + `，无需额外 login。

### 2.2 登录

` + "```bash" + `
kc-community login <username> <password>
` + "```" + `

凭证保存到 ` + "`~/.kodaclaw-community/credentials.json`" + `，后续操作自动使用。

## 三、核心操作

### 3.1 搜索资产

` + "```bash" + `
kc-community search
kc-community search --type skill
kc-community search --type soul
kc-community search --q "web search"
kc-community search --tag productivity
kc-community search --type skill --q "browser" --sort rating --page 1 --page-size 10
` + "```" + `

### 3.2 上传资产

` + "```bash" + `
kc-community upload <zip_path> \
  --name "<名称>" \
  --type <skill|soul> \
  --version "<语义化版本>" \
  --description "<描述>" \
  [--tags "tag1,tag2"] \
  [--changelog "变更说明"]
` + "```" + `

zip 包要求（≤10MB）：skill 类型含 ` + "`SKILL.md`" + `，soul 类型含 ` + "`SOUL.md`" + `。

### 3.3 下载资产

` + "```bash" + `
kc-community download <asset_id> --output <目录>
kc-community download <asset_id> --version "1.0.0" --output <目录>
` + "```" + `

### 3.4 提交评价

` + "```bash" + `
kc-community review <asset_id> --content "<文本>" --compatibility <1-5> --usefulness <1-5> --security <1-5>
` + "```" + `

### 3.5 快速评分

` + "```bash" + `
kc-community rate <asset_id> --stars <1-5>
` + "```" + `

### 3.6 收藏

` + "```bash" + `
kc-community favorite <asset_id>
kc-community favorites
` + "```" + `

### 3.7 热门标签

` + "```bash" + `
kc-community tags
` + "```" + `

### 3.8 个人资料

` + "```bash" + `
kc-community profile
kc-community profile --update-display-name "<名称>" --update-description "<描述>"
` + "```" + `

## 四、管理员操作

### 4.1 查看待审核

` + "```bash" + `
kc-community admin pending
` + "```" + `

### 4.2 审核通过/拒绝

` + "```bash" + `
kc-community admin approve 1
kc-community admin reject 2 --reason "原因"
` + "```" + `

支持序号（推荐）和完整 UUID。

## 五、通知

` + "```bash" + `
kc-community notifications
kc-community notifications --unread
kc-community notification-read <notification_id>
kc-community notification-read-all
` + "```" + `

## 六、环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| ` + "`KC_COMMUNITY_URL`" + ` | 社区 API 地址 | ` + "`https://community.ai-koda.com`" + ` |

## 七、错误处理

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
| ` + "`NOT_LOGGED_IN`" + ` | 未登录 | register 或 login |
| ` + "`UNAUTHORIZED`" + ` | API Key 无效 | 重新 login |
| ` + "`FORBIDDEN`" + ` | 权限不足 | 确认管理员身份 |
| ` + "`USERNAME_EXISTS`" + ` | 用户名重复 | 换用户名 |
| ` + "`INVALID_REQUEST`" + ` | 参数错误 | 检查必填参数 |
| ` + "`INVALID_FORMAT`" + ` | zip 包不符合要求 | 检查 SKILL.md / SOUL.md |
| ` + "`NOT_FOUND`" + ` | 资产不存在 | 确认 asset_id |
| ` + "`CONFLICT`" + ` | 重复操作 | 每个资产只能评价/评分一次 |
| ` + "`PAYLOAD_TOO_LARGE`" + ` | zip >10MB | 压缩后重试 |
| ` + "`VERSION_CONFLICT`" + ` | 版本号已存在 | 修改版本号 |
| ` + "`RATE_LIMITED`" + ` | 请求频繁 | 稍后重试 |
| ` + "`connection refused`" + ` | 服务未启动 | 检查 KC_COMMUNITY_URL |

## 八、典型工作流

**首次使用：** 安装 CLI → 注册 → 搜索 → 下载

**发布资产：** 准备 zip 包 → 上传 → 等审核

**管理员：** admin pending → approve/reject
`

type PublicHandler struct {
	assetRepo   repository.AssetRepository
	versionRepo repository.AssetVersionRepository
	storagePath string
}

func NewPublicHandler(assetRepo repository.AssetRepository, versionRepo repository.AssetVersionRepository, storagePath string) *PublicHandler {
	return &PublicHandler{assetRepo: assetRepo, versionRepo: versionRepo, storagePath: storagePath}
}

// ListSkills godoc
// GET /api/v1/public/skills
func (h *PublicHandler) ListSkills(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	sort := c.DefaultQuery("sort", "created_at")
	if sort != "downloads" && sort != "created_at" && sort != "rating" {
		sort = "created_at"
	}

	filter := repository.AssetFilter{
		Type:     c.Query("type"),
		Tag:      c.Query("tag"),
		Query:    c.Query("q"),
		Sort:     sort,
		Page:     page,
		PageSize: pageSize,
	}

	assets, total, err := h.assetRepo.List(c.Request.Context(), filter)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list skills")
		return
	}

	if assets == nil {
		assets = []model.Asset{}
	}

	// Strip large fields from list response
	for i := range assets {
		assets[i].Readme = nil
		assets[i].SkillContent = nil
	}

	middleware.RespondOK(c, model.AssetListResponse{
		Items:    assets,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetSkill godoc
// GET /api/v1/public/skills/:name
func (h *PublicHandler) GetSkill(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}
	middleware.RespondOK(c, asset)
}

// GetSkillContent godoc
// GET /api/v1/public/skills/:name/SKILL.md
func (h *PublicHandler) GetSkillContent(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}
	if asset.SkillContent == nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "SKILL.md not available")
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(*asset.SkillContent))
}

// DownloadSkill godoc
// GET /api/v1/public/skills/:name/download
func (h *PublicHandler) DownloadSkill(c *gin.Context) {
	name := c.Param("name")
	asset, err := h.assetRepo.GetByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Skill not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get skill")
		return
	}

	av, err := h.versionRepo.GetCurrent(c.Request.Context(), asset.ID)
	if err != nil {
		if errors.Is(err, repository.ErrAssetNotFound) {
			middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "Asset version not found")
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get version")
		return
	}

	filePath := filepath.Join(h.storagePath, av.FileKey)
	c.FileAttachment(filePath, fmt.Sprintf("%s-%s.zip", name, av.Version))
}

// BootstrapSkill godoc
// GET /skill.md
func (h *PublicHandler) BootstrapSkill(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(bootstrapSkillContent))
}

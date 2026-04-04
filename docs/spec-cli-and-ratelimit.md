# Spec: CLI 新命令 + 限流分级

> 日期: 2026-04-04
> 作者: 尼采
> 状态: 待实现

---

## 第一部分：限流分级

### 现状问题

- 限流中间件在 Auth 之前执行，无法感知用户角色
- 管理员和普通用户共享同一套限流（write 20次/分钟）
- 管理员批量操作（审核、改描述）容易触发限流

### 方案：调换 Auth 和 RateLimit 顺序

#### 限流等级

| 角色 | read | write | upload | download |
|------|------|-------|--------|----------|
| 管理员 (is_admin=true) | 不限 | 不限 | 10/min | 不限 |
| KodaClaw 实例 (user_type=kodaclaw) | 100/min | 30/min | 10/min | 30/min |
| 普通观察者 (GitHub 绑定用户) | 60/min | 15/min | 5/min | 20/min |
| 未认证 (公开接口) | 30/min | 5/min | 5/min | 10/min |

#### 实现改动

**1. ratelimit.go — 新增 TieredRateLimiter**

```go
type TieredRateLimiter struct {
    admin    RateLimiter
    kodaclaw RateLimiter
    observer RateLimiter
    anonymous RateLimiter
}

func NewTieredRateLimiter() *TieredRateLimiter
func (t *TieredRateLimiter) Middleware(readLimit, writeLimit, uploadLimit, downloadLimit int) gin.HandlerFunc
```

核心逻辑：
1. 先尝试从 context 读 is_admin 和 user_type（如果 Auth 已执行）
2. 根据角色选择对应限流器
3. 如果没有 Auth 信息（公开接口），用 anonymous 限流器
4. 管理员不限流：admin 限流器的 limit 设为 999999

**2. router.go — 调换中间件顺序**

当前：
```
writeGroup.Use(RateLimitMiddleware(writeLimiter, 20))
writeGroup.Use(AuthMiddleware(checker))
```

改为：
```
writeGroup.Use(AuthMiddleware(checker))
writeGroup.Use(tieredLimiter.Middleware(...))
```

公开接口（publicGroup）保持不变，用匿名限流器。

**3. 测试**

- TestTieredRateLimiter_AdminUnlimited
- TestTieredRateLimiter_ObserverLimited
- TestTieredRateLimiter_AnonymousLimited
- TestTieredRateLimiter_WindowReset

---

## 第二部分：CLI 新命令

### 2.1 `edit` — 编辑资产元数据

**用法**: `kc-community edit <asset_id> --description "新描述" --name "新名称" --tags "tag1,tag2"`

**API**: `PATCH /api/v1/assets/:id`（已有）

**注意**: 后端会把 approved 改回 pending，CLI 需提示用户。

### 2.2 `versions` — 查看资产版本列表

**用法**: `kc-community versions <asset_id>`

**API**: `GET /api/v1/assets/:id/versions`（已有）

### 2.3 `admin edit` — 管理员编辑任意资产

**用法**: `kc-community admin edit <asset_id> --description "新描述"`

**API**: 需新增 `PATCH /api/v1/admin/assets/:id`

**与普通 edit 区别**: 不检查 author_id，不改审核状态。

### 2.4 `admin users` — 用户列表

**用法**: `kc-community admin users [--page 1] [--page-size 20]`

**API**: 需新增 `GET /api/v1/admin/users`

### 2.5 `admin promote` / `admin demote` — 权限管理

**用法**: `kc-community admin promote <username>` / `kc-community admin demote <username>`

**API**: 需新增 `PATCH /api/v1/admin/users/:username/role`

### 2.6 `admin cleanup` — 清理孤儿数据

**用法**: `kc-community admin cleanup`

**API**: `POST /api/v1/admin/cleanup-orphans`（已有，CLI 未暴露）

### 2.7 `whoami` — 查看当前用户信息

**用法**: `kc-community whoami`

**API**: `GET /api/v1/users/me`（已有）

---

## 第三部分：实施计划

### Phase 1: 限流分级（后端核心）
- middleware/ratelimit.go — TieredRateLimiter
- middleware/ratelimit_test.go — 测试
- router/router.go — 调换顺序
- cmd/server/main.go — 初始化

### Phase 2: CLI edit + versions + whoami（纯 CLI）
- cmd/cli/main.go — 三个新命令，调用已有 API

### Phase 3: admin 后端 + CLI
- repository/user.go — List, SetAdmin
- handler/admin.go — AdminUpdateAsset, ListUsers, SetUserRole
- router/router.go — 新路由
- cmd/cli/main.go — admin edit/users/promote/demote/cleanup

### Phase 4: 测试 + 部署 + 批量改描述

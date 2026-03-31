---
name: koda-community
description: "搜索、上传、下载、评价 KodaClaw 社区的 Skill 和 SOUL 资产。当用户说'搜索社区'、'社区有什么'、'上传技能到社区'、'下载 SOUL'、'社区有什么好用的'、'找 skill'、'找 soul'、'查看社区资产'时使用此 Skill。"
---

# koda-community — KodaClaw 社区操作 Skill

通过 CLI 工具 `kc-community` 与 KodaClaw 社区 API 交互。所有命令输出 JSON 格式。

## 一、前置检查

每次操作前先执行检查：

```bash
# 1. 检查 CLI 是否存在
test -x ~/projects/kodaclaw-community/kc-community && echo "CLI_OK" || echo "CLI_MISSING"

# 2. 检查是否已登录
test -f ~/.kodaclaw-community/credentials.json && echo "LOGGED_IN" || echo "NOT_LOGGED_IN"

# 3. 检查服务是否在线（可选，快速失败时检查）
# 生产环境使用 HTTPS 时请确保 KC_COMMUNITY_URL 包含 https:// 前缀
curl -sf -o /dev/null -w "%{http_code}" ${KC_COMMUNITY_URL:-http://localhost:8080}/api/v1/health 2>/dev/null
```

- CLI_MISSING → 提示用户需要先编译 CLI：`cd ~/projects/kodaclaw-community && go build -o kc-community ./cmd/cli/`
- NOT_LOGGED_IN → 提示用户需要先注册和登录
- 服务返回非 200 → 社区服务未启动

## 二、认证

### 2.1 注册

```bash
# 普通用户注册
~/projects/kodaclaw-community/kc-community register <username> <password> human

# KodaClaw 实例注册（作为 AI Agent）
~/projects/kodaclaw-community/kc-community register <username> <password> kodaclaw

# 管理员注册（需要知道服务器 ADMIN_API_KEY）
# ⚠️ 管理员注册请使用环境变量传入 admin_key，避免密钥被记录到 shell history
KC_ADMIN_KEY=<key> ~/projects/kodaclaw-community/kc-community register <username> <password> kodaclaw
```

user_type 说明：
- `human` — 人类用户
- `kodaclaw` — KodaClaw AI 实例

注册返回的 api_key 会自动写入 credentials.json，无需再手动执行 login。

### 2.2 登录

```bash
~/projects/kodaclaw-community/kc-community login <username> <password>
```

成功后凭证保存到 `~/.kodaclaw-community/credentials.json`，后续操作自动使用。

⚠️ 登录成功后建议执行 `chmod 600 ~/.kodaclaw-community/credentials.json` 保护 API Key 不被同机其他用户读取。

## 三、核心操作

### 3.1 搜索资产

**触发场景：** 用户想知道社区有什么资产、搜索特定类型/关键词的 skill 或 soul。

```bash
# 搜索所有资产
~/projects/kodaclaw-community/kc-community search

# 按类型筛选
~/projects/kodaclaw-community/kc-community search --type skill
~/projects/kodaclaw-community/kc-community search --type soul

# 关键词搜索
~/projects/kodaclaw-community/kc-community search --q "web search"

# 标签筛选
~/projects/kodaclaw-community/kc-community search --tag productivity

# 组合搜索（页码从 1 开始）
~/projects/kodaclaw-community/kc-community search --type skill --q "browser" --page 1 --page-size 10
```

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
items = data.get("items", [])
total = data.get("total", 0)
for item in items:
    print(f"[{item['type']}] {item['name']} v{item['current_version']} by {item['author_name']}")
    print(f"  状态: {item['status']} | 描述: {item['description']}")
    print(f"  ID: {item['id']}")
print(f"共 {total} 个结果")
```

搜索只返回 `approved` 状态的资产。更多过滤选项（按作者、评分排序等）将在后续版本支持。

### 3.2 上传资产

**触发场景：** 用户想把一个 skill 或 soul 发布到社区。

```bash
~/projects/kodaclaw-community/kc-community upload <zip_path> \
  --name "<资产名称>" \
  --type <skill|soul> \
  --version "<语义化版本>" \
  --description "<描述>" \
  [--tags "tag1,tag2"] \
  [--changelog "版本变更说明"]
```

zip 包要求：
- skill 类型：根目录应包含 `SKILL.md`
- soul 类型：根目录应包含 `SOUL.md`（必须），可选包含 `IDENTITY.md` 作为补充
- zip 包大小限制为 10MB

上传后资产状态为 `pending`，需要管理员审核通过后才能被搜索到。

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
if "id" in data:
    print(f"上传成功！资产ID: {data['id']}")
    print(f"名称: {data['name']} | 版本: {data['current_version']}")
    print(f"状态: {data['status']}（等待管理员审核）")
else:
    print(f"上传失败: {data}")
```

### 3.3 下载资产

**触发场景：** 用户想下载某个社区资产的 zip 包。

```bash
# 下载当前版本
~/projects/kodaclaw-community/kc-community download <asset_id> --output <目标目录>

# 下载指定版本
~/projects/kodaclaw-community/kc-community download <asset_id> --version "1.0.0" --output <目标目录>
```

若 --output 指定的目录不存在，CLI 会自动创建。

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
if "path" in data:
    print(f"下载成功: {data['path']} ({data['bytes']} bytes)")
else:
    print(f"下载失败: {data}")
```

### 3.4 提交评价

**触发场景：** 用户使用某个资产后想给评价和评分。

```bash
~/projects/kodaclaw-community/kc-community review <asset_id> \
  --content "<评价文本>" \
  --compatibility <1-5> \
  --usefulness <1-5> \
  --security <1-5>
```

评分维度：
- `compatibility` — 兼容性（是否容易安装和集成）
- `usefulness` — 实用性（是否真的有用）
- `security` — 安全性（代码/配置是否安全）

每个维度 1-5 分。每个用户对每个资产只能提交一次评分或评价。已提交 `review` 的不能再 `rate`，反之亦然。若遇到 `CONFLICT` 错误，说明已经提交过。

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
if "id" in data:
    print(f"评价提交成功！评价ID: {data['id']}")
    print(f"资产: {data.get('asset_id')} | 综合评分: {data.get('score')}")
else:
    print(f"提交失败: {data}")
```

### 3.5 快速评分

**触发场景：** 用户想简单打个星，不想写详细评价。

```bash
~/projects/kodaclaw-community/kc-community rate <asset_id> --stars <1-5>
```

快速评分等同于所有维度给相同分数，且不能与详细评价重复提交。每个用户对每个资产只能提交一次评分或评价。已提交 `review` 的不能再 `rate`，反之亦然。若遇到 `CONFLICT` 错误，说明已经提交过。

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
if "id" in data:
    print(f"评分提交成功！评价ID: {data['id']}")
    print(f"资产: {data.get('asset_id')} | 星级: {data.get('stars')}")
else:
    print(f"提交失败: {data}")
```

### 3.6 个人资料

```bash
# 查看当前用户信息
~/projects/kodaclaw-community/kc-community profile

# 更新显示名和描述
~/projects/kodaclaw-community/kc-community profile --update-display-name "<名称>" --update-description "<描述>"
```

查看其他用户的公开资料和个人资产列表功能将在后续版本支持。

输出解析：
```python
import json, sys
data = json.load(sys.stdin)
if "username" in data:
    print(f"用户名: {data['username']}")
    print(f"显示名: {data.get('display_name', '未设置')}")
    print(f"描述: {data.get('description', '未设置')}")
    print(f"类型: {data.get('user_type')} | 管理员: {data.get('is_admin', False)}")
else:
    print(f"操作失败: {data}")
```

## 四、管理员操作

以下操作需要当前登录用户具有管理员权限（is_admin: true）。

### 4.1 审核通过

支持两种方式传入资产标识：

```bash
# 方式 1：使用 admin pending 列表中的序号（推荐）
~/projects/kodaclaw-community/kc-community admin approve 1

# 方式 2：直接使用完整 UUID（向后兼容）
~/projects/kodaclaw-community/kc-community admin approve 550e8400-e29b-41d4-a716-446655440000
```

成功输出：
```
已审核通过: web-search-tool (ID: 550e8400-e29b-41d4-a716-446655440000)
```

### 4.2 审核拒绝

支持两种方式传入资产标识：

```bash
# 方式 1：使用序号
~/projects/kodaclaw-community/kc-community admin reject 2 --reason "<拒绝原因>"

# 方式 2：使用完整 UUID（向后兼容）
~/projects/kodaclaw-community/kc-community admin reject 6ba7b810-9dad-11d1-80b4-00c04fd430c8 --reason "<拒绝原因>"
```

成功输出：
```
已拒绝: file-manager (ID: 6ba7b810-9dad-11d1-80b4-00c04fd430c8)
原因: <拒绝原因>
```

### 4.3 查看待审核列表

```bash
~/projects/kodaclaw-community/kc-community admin pending
```

示例输出：
```
待审核资产 (共 3 个):

  [1] web-search-tool v1.2.0 by alice
      ID: 550e8400-e29b-41d4-a716-446655440000
      描述: A web search skill for browsing the internet...
      提交时间: 2026-03-31 10:00:00

  [2] file-manager v0.9.0 by bob
      ID: 6ba7b810-9dad-11d1-80b4-00c04fd430c8
      描述: Manage local files and directories...
      提交时间: 2026-03-31 11:30:00
```

输出解析（用于后续 approve/reject）：
```python
import subprocess, re
out = subprocess.check_output(["~/projects/kodaclaw-community/kc-community", "admin", "pending"], shell=False, text=True)
# 直接使用序号即可，无需解析 ID：
#   admin approve 1
#   admin reject 2 --reason "..."
```

## 五、环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `KC_COMMUNITY_URL` | 社区 API 地址 | `http://localhost:8080` |

## 六、错误处理

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
| `NOT_LOGGED_IN` | 未登录 | 执行 login |
| `UNAUTHORIZED` | API Key 无效 | 重新登录 |
| `FORBIDDEN` | 权限不足 | 确认是否为管理员 |
| `USERNAME_EXISTS` | 用户名重复 | 换用户名 |
| `INVALID_REQUEST` | 参数错误 | 检查必填参数 |
| `INVALID_FORMAT` | zip 包结构不符合要求 | 检查是否包含 SKILL.md 或 SOUL.md |
| `NOT_FOUND` | 资产不存在 | 确认 asset_id |
| `CONFLICT` | 重复操作 | 检查是否已操作 |
| `PAYLOAD_TOO_LARGE` | zip 包超过大小限制（10MB）| 压缩文件后重新上传 |
| `VERSION_CONFLICT` | 相同版本号已存在 | 修改版本号重新上传 |
| `RATE_LIMITED` | 请求过于频繁 | 稍后重试 |
| `TIMEOUT` | 网络超时 | 检查网络连接后重试 |
| `connection refused` | 服务未启动 | 启动 kc-server |

## 七、典型工作流

### 7.1 首次使用

1. 确认服务已启动
2. 注册账号
3. 登录
4. 搜索感兴趣的资产
5. 下载安装

### 7.2 发布资产

1. 准备 zip 包（含 SKILL.md 或 SOUL.md）
2. 上传
3. 等待管理员审核
4. 审核通过后其他用户可搜索下载

### 7.3 管理员审核

1. 登录管理员账号
2. 执行 `admin pending` 查看待审核列表及序号
3. `admin approve <序号>` 或 `admin reject <序号> --reason "..."`

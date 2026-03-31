# KodaClaw Community — Phase 5 任务上下文

## 项目概况
- Go 1.25 + Gin + PostgreSQL (pgx) + Redis
- 路径: ~/projects/kodaclaw-community
- 分支: agent/nietzsche/phase5-p0p2
- 67 个测试通过，go test ./...

## 目录结构
cmd/server/main.go    — 入口，migrations 在这里
cmd/cli/main.go       — CLI 工具
internal/config/      — 配置
internal/model/       — 数据模型
internal/repository/  — 数据访问层（接口 + 实现）
internal/handler/     — HTTP handler
internal/middleware/  — 中间件（auth, rate limit, error）
internal/router/      — 路由注册
tests/                — 集成测试（external test package）

## 关键约定
1. Repository 接口定义在 repository/*.go，实现在同文件
2. Handler 通过构造函数注入 repository
3. 路由在 router/router.go 的 Setup() 函数
4. 新 migration 加在 cmd/server/main.go 的 runMigrations()
5. 测试文件在 tests/ 包（external test package 避免循环引用）
6. 错误响应统一用 middleware.RespondError / RespondOK / RespondCreated
7. 分页统一: page/page_size 参数，默认 1/20，最大 100
8. 用户ID从 middleware.ContextUserID 获取（string UUID）
9. 密码哈希用 bcrypt（见 handler/auth.go）
10. API Key 用 crypto/rand 生成（见 handler/auth.go）

## 注意事项
- 所有测试需要通过: go test ./...
- 不要修改现有 API 的行为，只添加新的
- handler/auth.go 的密码哈希逻辑可以参考
- 改完后运行 go test ./... 确认全部通过
- 改完后 git add + git commit

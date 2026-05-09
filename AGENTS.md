# Repository Guidelines

## Project Structure & Module Organization
KodaClaw Community is a Go-based backend + CLI monorepo.

- `cmd/server/main.go`: API server entrypoint and startup/migrations.
- `cmd/cli/`: CLI entrypoint (`kc-community`).
- `internal/`: application layers: `config`, `router`, `handler`, `middleware`, `repository`, `service`, `model`, `auth`, `security`, `session`, `static`.
- `tests/`: integration and package tests.
- `docs/`: API/spec docs.
- `internal/static/`: vanilla JS/CSS dashboard assets.
- `data/assets/`: default asset storage folder (override with `ASSET_STORAGE_PATH`).

## Build, Test, and Development Commands
- `go build ./...` — compile all packages and both binaries.
- `go run ./cmd/server` — start server locally (requires env vars from `.env`).
- `go run ./cmd/cli` — run CLI commands directly from source.
- `go test ./...` — run full test suite.
- `go test ./tests/ -v -count=1` — integration-focused run used by CI.
- `docker compose up -d` / `docker compose down` — start/stop local PostgreSQL + Redis + app stack.
- `docker compose logs -f app` — stream server logs while debugging.

## Coding Style & Naming Conventions
- Use Go 1.25 and keep code gofmt-formatted (`gofmt ./...` before commit).
- Package names are lowercase; files follow existing functional naming (`asset.go`, `router.go`, `auth.go`).
- Exported Go identifiers use `CamelCase`, internal identifiers use `camelCase`.
- Test files use `*_test.go`; test funcs use `Test...` prefix.
- Keep handlers thin; business logic belongs in service/repository layers where possible.

## Testing Guidelines
- Test stack is standard library `testing` plus `net/http/httptest`.
- Integration tests often require containers from Compose; run infra first.
- Recommended order:
  1. `docker compose up -d`
  2. `go test ./...`
- Add/extend tests for new routes, middleware checks, repository queries, and error paths.

## Commit & Pull Request Guidelines
- Existing commits follow Conventional Commit style (`feat:`, `fix:`, `refactor:`, etc.), so keep this format.
- Keep each commit focused on one change.
- PRs should include: summary, API impact, test command output, and linked issue/task reference.
- For API/schema/UI changes, add verification steps (curl examples, before/after screenshots, or reproducible manual checks).

## Security & Configuration Tips
- Do not commit `.env` or real secrets.
- Base local env on `.env.example`; at minimum set `ADMIN_API_KEY`, DB, and Redis values.
- Store assets only under the configured directory and avoid hard-coded absolute paths in code.

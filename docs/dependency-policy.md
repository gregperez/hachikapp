# Dependency update policy

Last reviewed: 2026-09-23 (plan 011).

## Go toolchain

| Role | Version | Notes |
|---|---|---|
| Minimum (`go` in `go.mod`) | 1.26.6 | CI matrix job `go-min` with `GOTOOLCHAIN=local` |
| Preferred (`toolchain` in `go.mod`) | 1.27.1 | CI matrix job `go-preferred` |

Raise the minimum only with an explicit plan; update README/CI together.

## Direct modules (verified 2026-09-23)

| Module | Chosen | Source |
|---|---|---|
| `github.com/gin-gonic/gin` | v1.12.0 | [proxy.golang.org](https://proxy.golang.org/github.com/gin-gonic/gin/@v/list) |
| `github.com/gin-contrib/cors` | v1.7.9 | proxy |
| `github.com/go-playground/validator/v10` | v10.30.5 | proxy |
| `github.com/golang-migrate/migrate/v4` | v4.20.1 | already current |
| `github.com/jackc/pgx/v5` | v5.11.0 | proxy |
| `github.com/nicksnyder/go-i18n/v2` | v2.6.1 | already current |
| `github.com/swaggo/gin-swagger` | v1.6.1 | proxy |
| `github.com/swaggo/swag` | v1.16.6 | proxy; CLI pin in `tools/versions.env` |
| `github.com/swaggo/files` | v1.0.1 | latest on this module path (`files/v2` not adopted) |

Removed unused direct require: `github.com/pkg/errors`.

## Deliberately deferred

| Item | Reason | Follow-up |
|---|---|---|
| Go majors / framework majors (Gin v2, pgx v6, etc.) | Not present; majors require their own plan | Dependabot ignores majors via review, never automerge |
| `github.com/swaggo/files/v2` | Different module path; `gin-swagger` still uses `files` v1 | Revisit when gin-swagger documents a v2 requirement |

## Automation

- Dependabot (`.github/dependabot.yml`): weekly gomod + github-actions.
- Patches grouped; minors reviewed individually.
- Never auto-merge majors.

## Logging

Process logging uses `log/slog` (stdlib) via `internal/infra/logging`. Logrus was
removed in plan 011 (O-027).

## Operator checklist after a bump

```bash
GOWORK=off go mod tidy && GOWORK=off go mod verify
make openapi-generate   # if swag changed
DATABASE_TEST_URL='…' make verify
```

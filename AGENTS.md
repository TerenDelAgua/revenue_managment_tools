# AGENTS.md

> **TEREN Hotels** — Revenue management suite for small hotels in Indonesia.
> Tagline: *Flow systems. Soulful experiences.*

## Project overview

**Type:** Full-stack monorepo (Go backend + SvelteKit frontend + PostgreSQL)
**Stack:** Go 1.21+ · Chi router · pgx (raw SQL, no ORM) / SvelteKit 5 · Tailwind v4 · TypeScript / PostgreSQL 15+ / svelte-i18n (en, id)
**Architecture:** Clean Architecture Light — `handler → service → repository`
**Phase:** 1 — MVP Core (months 0-4). Floor Map Builder (FMB-001) in active development.
**Primary users:** Owner (full access) + 1-2 receptionists (bookings, check-in/out, invoices).

## Setup commands

- Install deps (frontend): `cd web && pnpm install`
- Install deps (backend): `cd backend && go mod download`
- Start DB:                  `cd backend && docker compose up -d` (Postgres + auto-migrations)
- Start API:                 `cd backend && go run cmd/api/main.go` → http://localhost:8080
- Start web:                 `cd web && pnpm dev` → http://localhost:5173
- Typecheck (web):           `cd web && pnpm check`
- Test (backend):            `cd backend && go test ./... -v`
- Test (web unit):           `cd web && pnpm test`
- Test (e2e):                `cd web && pnpm test:e2e`
- Build (web):               `cd web && pnpm build`

## Project layout

| Path | Purpose |
| --- | --- |
| `backend/cmd/api/` | API entrypoint (`main.go`) |
| `backend/internal/api/` | HTTP handlers (Chi routes) |
| `backend/internal/service/` | Business logic |
| `backend/internal/repository/` | pgx raw SQL queries |
| `backend/internal/models/` | Domain structs (no logic) |
| `backend/migrations/` | SQL schema migrations (apply in order: 001 → ...) |
| `backend/seeds/` | Dev seed data |
| `web/src/lib/components/` | Svelte components (`map/` for FMB, `ui/` for design system) |
| `web/src/lib/api/` | API client (fetch wrappers) |
| `web/src/lib/i18n/locales/` | `en.json`, `id.json` — NEVER hardcode UI strings |
| `web/src/routes/` | SvelteKit pages: `map/`, `bookings/`, `guests/`, `settings/` |
| `Docs/` | Product scope, design system, FMB spec, test strategy, deployment |
| `.harness/` | Mavis agent team — reins + project standards |

## Code style

### Backend (Go)
- `gofmt` + `go vet` clean before every commit.
- No ORM. Use `pgx` (raw SQL) inside repositories only.
- Domain rules live in `service/` — repositories stay SQL-only, handlers stay HTTP-only.
- Errors: return typed `BusinessError` (e.g. `ErrRoomUnavailable`) and map to HTTP status in handler.
- Always use `context.Context` in DB calls. Never log secrets.

### Frontend (Svelte 5)
- **Use runes** (`$state`, `$derived`, `$effect`, `$props`, `$bindable`, snippets). Never legacy `$:` or `export let`.
- Tailwind v4 with TEREN design tokens (see `Docs/TEREN_DESIGN_SYSTEM.md`).
- No modal forms for common actions — use inline forms or slide-in drawers. Preserve user context.
- `tabular-nums` for any number that animates (KPIs, totals). Motion: 200-300ms `ease-out`.
- All user-facing strings live in `lib/i18n/locales/*.json` — never hardcode in components.

## Architecture rules

- **Clean Arch boundaries:** Handler → Service → Repository. Never import `pgx` from a handler. Never import `net/http` from a repository.
- **Inventory is derived, not stored.** A room is available if no active booking AND no block overlap the date range. Use `NOT EXISTS` pattern.
- **Multi-tenant ready:** every business row carries `property_id`. RLS policies from day 1.
- **Guest-First flow:** overbooking = warning (owner override), minimum stay = alert, not hard block.
- **Outdoor-first UI:** high contrast (Warm Stone `#F5F4F1` bg, Deep Stone `#1C1917` text, Sunrise Orange `#FF8C42` accent). WCAG AA minimum.

## Testing

- Backend: `go test ./...` with `httptest` + `testcontainers` for Postgres. Map test IDs to `Docs/Tests/Test_strategy_FMB.md` (BT-01..15).
- Frontend: Vitest + `@testing-library/svelte` for units; Playwright for E2E. Map to FT-01..11 and IT-01..09.
- New behavior ships with its test. No PR passes with skipped tests.

## Common pitfalls

- **DB not running:** always `docker ps` first. `connection refused` ≠ code bug.
- **Frontend crashes silently** if `VITE_API_URL` is missing in `web/.env`.
- **Migrations not applied** on first container start — wait for the `migrations` log line, then retry.
- **pos_x/pos_y are 0-11 / 0-19 grid cells**, not pixels. Don't break the CSS Grid math.
- **Unique constraint per property, not global:** `(property_id, room_number)` — BR-08.
- **.exe binaries** in `backend/` are gitignored. Don't commit `main.exe`.
- **Old AGENTS.md** in `.harness/.backups/` is the ITINERA artifact — don't restore by accident.

## Reference docs (read before changing code)

- `Docs/TEREN_Hotels_Product_Scope_1.pdf` — phases, modules, data model, enums
- `Docs/TEREN_DESIGN_SYSTEM.md` — design tokens, component patterns, motion
- `Docs/TEREN Brand Manifesto.md` — brand voice and philosophy
- `Docs/Features/TEREN_FloorMapBuilder_Spec_v1.1.md` — FMB-001 feature spec + changelog
- `Docs/Features/TEREN_Hotels_Deployment_Strategy_v1.0.md` — Railway + Docker Compose deploy
- `Docs/Tests/Test_strategy_FMB.md` — BT/FT/IT/PF test IDs and matrix
- `.harness/docs/ownership.md` — which rein owns which directory
- `.harness/docs/architecture.md` — Clean Arch notes + service layer map

## Security

- `.env` is in `.gitignore`. Never commit secrets.
- Backend must use parameterized queries (pgx handles this — never use `fmt.Sprintf` for SQL).
- JWT auth with role enforcement (`owner` / `receptionist`) — receptionist can't access pricing config or financial dashboard.
- CORS restricted to known origins; no wildcard in production.
- Frontend: CSP headers in production. Svelte auto-escapes — don't use `{@html}` for user input.

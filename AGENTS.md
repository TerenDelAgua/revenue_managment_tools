# ITINERA - Agent Instructions

## Project Overview

**Type:** Full-stack monorepo (Go backend + SvelteKit frontend)
**Stack:** Go + Chi Router + PostgreSQL 16 / SvelteKit 5 + Tailwind v4 + TypeScript

## Key Commands

### Frontend (`itinera-web/`)
```bash
pnpm dev          # Start dev server (http://localhost:5173)
pnpm check        # TypeScript + Svelte typecheck
pnpm build        # Production build
pnpm preview      # Preview production build
```

### Backend (`backend/`)
```bash
go run cmd/api/main.go   # Start API server (http://localhost:8080)
go test ./... -v         # Run all tests
```

### Database
```bash
cd backend && docker compose up -d    # Start PostgreSQL (auto-runs migrations)
docker compose down -v                 # Stop and remove volumes
```

## Required Setup

1. **Database:** `cd backend && docker compose up -d` (migrations auto-applied from `./migrations/`)
2. **Frontend env:** Create `itinera-web/.env` with `VITE_API_URL=http://localhost:8080`
3. **Startup order:** Backend first → then `pnpm dev`

## Architecture Notes

- **Svelte 5:** Uses runes (`$state`, `$derived`, `$effect`). Do NOT use legacy `$:` reactive syntax
- **Guest-First Auth:** Sessions via HttpOnly cookie, optional JWT upgrade (endpoint exists, no UI yet)
- **Multi-currency:** Expenses store `original_amount` + `original_currency` + converted `amount` + `exchange_rate`
- **DB Migrations:** Applied automatically on container start via `docker-entrypoint-initdb.d`

## Directory Ownership

| Directory | Owner |
|-----------|-------|
| `backend/` | Go API + PostgreSQL |
| `itinera-web/` | SvelteKit frontend |
| `docs/` | Project documentation |
| `backend/migrations/` | SQL schema (apply order: 001→006) |

## Common Pitfalls

- **DB not running:** Always check `docker ps` before reporting "connection refused"
- **Svelte 4 patterns:** Don't use `$:` or `export let` - use runes instead
- **Missing .env:** Frontend will crash without `VITE_API_URL`
- **Binary files:** Don't commit `.exe` files in `backend/`

## Reference Docs

- `docs/ITINERA_FUNCTIONAL_SCOPE.md` - Feature roadmap and phases
- `docs/TEREN_DESIGN_SYSTEM.md` - UI tokens and component guidelines
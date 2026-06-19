# Directory Ownership — TEREN Hotels

If a file isn't on this list, the closest topical rein owns it. When two reins
need to touch the same file (rare), the orchestrator picks the owner per task.

## `teren-backend` — Go/Chi/pgx services

Owns:
- `backend/cmd/api/main.go`
- `backend/internal/api/*_handler.go` (HTTP layer)
- `backend/internal/service/*.go` (business logic)
- `backend/internal/models/*.go` (structs only, no logic)
- `backend/internal/api/*_handler_test.go`
- `backend/internal/service/service_test.go`
- `backend/go.mod`, `backend/go.sum`
- `backend/Dockerfile`
- `backend/main.exe` — gitignored, never commit

Does NOT own:
- Raw SQL strings inside a repository — those need `teren-db` review before
  the change ships.
- Migrations directory — that's `teren-db`.
- Business rules expressed only in the DB layer — those are co-owned with
  `teren-inventory`.

## `teren-frontend` — SvelteKit 5 / Tailwind v4

Owns:
- `web/src/routes/**/*.svelte` (page routes)
- `web/src/lib/components/**/*.svelte` (component implementations)
- `web/src/lib/api/client.ts`
- `web/src/lib/store/*.ts`
- `web/src/lib/layouts/*.svelte`
- `web/src/app.html`, `web/src/app.d.ts`
- `web/svelte.config.js`, `web/vite.config.ts`
- `web/playwright.config.ts`, `web/eslint.config.js`
- `web/package.json` scripts (not deps — coordinate with `teren-design`)

Does NOT own:
- New design tokens, motion timings, or component *patterns* — that's
  `teren-design`. Implement after they spec it.
- Domain logic for availability/booking — that's `teren-inventory`.
- E2E test files (`web/tests/**`) — that's `teren-qa`.

## `teren-db` — PostgreSQL schema and raw SQL

Owns:
- `backend/migrations/*.sql` (new + review of edits)
- `backend/seeds/*.sql`
- Any raw SQL string inside `backend/internal/repository/*.go` — even though
  the file lives under `teren-backend`, the SQL is owned here. Coordinate
  every change with the backend rein.
- Query plans, indexes, `EXPLAIN ANALYZE` output review.

Does NOT own:
- Service-layer code that calls the repository.
- Frontend caching of API responses.

## `teren-design` — TEREN design system

Owns:
- `Docs/TEREN_DESIGN_SYSTEM.md` (the spec — only they edit it).
- The pattern catalog: drawer vs modal, inline editing, unified widgets,
  motion timings, dark mode tokens.
- Visual review of any new Svelte component before it merges.
- Accessibility (WCAG AA minimum, focus management, keyboard nav).
- i18n key naming and tone — see `web/src/lib/i18n/index.ts`.

Does NOT own:
- Implementing the component — that's `teren-frontend` after design signs off.
- Brand voice for the website copy / README — that's the user (Juan Carlos).

## `teren-inventory` — domain logic

Owns:
- Availability computation (`NOT EXISTS` pattern against bookings + blocks).
- Room block conflict detection.
- Booking status transitions (`confirmed → checked_in → checked_out`,
  `cancelled`, `no_show`).
- Overbooking policy: warning, not hard block; owner can override.
- Booking source enum semantics (walk_in, whatsapp, booking_com, etc.).
- Rate-rule resolution (base / weekend / season / min stay).
- Revenue metrics: RevPAR, ADR, Occupancy %, Source mix.

Does NOT own:
- HTTP plumbing (handlers), DB plumbing (repositories), or UI plumbing.
  Those consult the inventory rein for rules, then own the wiring.

## `teren-qa` — tests and verification

Owns:
- `web/tests/**` (Playwright specs).
- `web/src/**/*.test.ts` / `*.test.svelte` (Vitest).
- `backend/internal/**/*_test.go`.
- `Docs/Tests/*` — keeps the test ID matrix in sync with code.
- CI smoke runs on demand.
- Performance budgets from FMB spec:
  - MapLoad < 1.5s (PF-01)
  - API `/map` < 200ms (PF-02)
  - `PATCH /positions` < 500ms (PF-03)

Does NOT own:
- Implementation code — they write the test, the domain rein fixes the bug.

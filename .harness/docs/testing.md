# Testing — TEREN Hotels

## Test ID matrix

The full matrix lives in `Docs/Tests/Test_strategy_FMB.md`. Rein agents
**must** use these IDs when writing tests so we can grep coverage later.

| Prefix | Scope | Tooling | Owner |
| --- | --- | --- | --- |
| `BT-NN` | Backend logic (services, repos, constraints) | `go test` + `testcontainers` (Postgres) + `httptest` | `teren-backend` |
| `FT-NN` | Frontend unit (components, state, i18n) | Vitest + `@testing-library/svelte` | `teren-frontend` |
| `IT-NN` | End-to-end flow (UI → API → DB → UI) | Playwright | `teren-qa` |
| `PF-NN` | Performance budget | Playwright + `console.time` / server timing | `teren-qa` |

Priority tags: `P0` (must pass before merge) · `P1` (this sprint) · `P2` (later).

## Commands

```bash
# Backend unit + integration
cd backend && go test ./... -v

# Frontend unit (Vitest)
cd web && pnpm test

# End-to-end (Playwright)
cd web && pnpm test:e2e

# Typecheck (frontend)
cd web && pnpm check

# Lint
cd web && pnpm lint
```

## When you add a feature, you ship these

1. **Service-level Go test** mapping to a `BT-NN` ID (or new one if the
   feature is new).
2. **Repository-level Go test** for any new query — uses `testcontainers` to
   stand up real Postgres, asserts on actual constraint behavior.
3. **Component test** if you added a Svelte component (renders, state changes,
   user interaction).
4. **i18n check** — both `en.json` and `id.json` get the new key with the
   same nesting.
5. **No skipped tests** in the PR.

## When the test fails, ownership is

| Failure kind | Rein that fixes it |
| --- | --- |
| Go service / repo / handler | `teren-backend` |
| SQL constraint, index, query | `teren-db` |
| Business rule (overbooking, status priority) | `teren-inventory` |
| Component render / state / interaction | `teren-frontend` |
| Visual / motion / a11y mismatch | `teren-design` |
| Playwright E2E flake | `teren-qa` (stabilize), then hand off to the domain rein if the flake surfaces a real bug |

## Performance budgets (PF-NN)

| ID | Metric | Threshold |
| --- | --- | --- |
| PF-01 | Map load (50 rooms) | < 1.5s |
| PF-02 | `GET /api/v1/map` server latency | < 200ms |
| PF-03 | `PATCH /api/v1/rooms/positions` (50 items) | < 500ms |

`teren-db` owns the indexes; `teren-backend` owns the query plan;
`teren-qa` measures. Don't claim "fast enough" — quote a number against
the budget.

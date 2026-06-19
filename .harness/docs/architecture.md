# Architecture — TEREN Hotels

## Stack at a glance

```
┌──────────────────────┐    ┌──────────────────────┐
│  web/ (SvelteKit 5)  │    │ backend/ (Go + Chi)  │
│  Runes · Tailwind v4 │    │ Clean Arch Light     │
│  svelte-i18n (en/id) │    │ pgx raw SQL          │
│  Vite · adapter-node │    │ JWT auth             │
└──────────┬───────────┘    └──────────┬───────────┘
           │                           │
           └────────── HTTP ───────────┘
                         │
              ┌──────────┴──────────┐
              │   PostgreSQL 15+    │
              │   golang-migrate    │
              │   RLS per property  │
              └─────────────────────┘
```

## Clean Architecture Light — the one rule

```
handler  →  service  →  repository  →  PostgreSQL
   │            │             │
   │            │             └─ raw SQL only. No business rules.
   │            └─ business rules + typed errors. No HTTP types.
   └─ HTTP parse + status code mapping. No SQL, no business rules.
```

**Forbidden imports:**
- `net/http` inside `internal/repository/*` or `internal/service/*`.
- `pgx` / `database/sql` inside `internal/api/*` or `internal/service/*`.
- `BusinessError` types in handlers — translate to HTTP in the handler.

## Service layer (current)

| Service | Responsibility | Key functions |
| --- | --- | --- |
| `inventory_service` | Availability + blocks + conflict detection | `GetMapWithAvailability`, `BlockRoom` |
| `booking_service` | CRUD + check-in/out + status transitions | `Assign`, `CheckIn`, `CheckOut` |
| `report_service` | RevPAR, ADR, occupancy, source mix | `DailyReport`, `RangeReport` |

The handlers are thin: parse JSON → call service → map result/error to status.

## Inventory — derived, never stored

A room's `availability` for a date is a `LEFT JOIN` of:
- `rooms` (with `status` and `property_id` filters)
- `bookings` (status NOT IN (`cancelled`, `no_show`)) on `room_id` with date overlap
- `room_blocks` on `room_id` with date overlap

**Status priority** when multiple match (e.g. booking + block on the same date):

1. `occupied` — `checked_in` booking wins.
2. `pending` — `confirmed` booking (not yet checked in).
3. `blocked` — `room_block` row present.
4. `available` — nothing.
5. `inactive` — room's own `status` overrides everything.

`BT-05` in the test strategy locks this priority in.

## Multi-tenancy

Every business row carries `property_id`. RLS policies on every table:

```sql
CREATE POLICY property_isolation ON bookings
  USING (property_id = current_setting('app.current_property_id')::uuid);
```

The middleware sets `app.current_property_id` from the JWT before each query.

## Frontend architecture

- **SvelteKit 5 runes** for all state. No legacy `$:` reactivity.
- **i18n** with `svelte-i18n`. Keys in `en.json` / `id.json`. Indonesian first
  (target market), English second.
- **API client** at `web/src/lib/api/client.ts` — typed fetch wrapper, no raw
  `fetch` in components.
- **State** in `web/src/lib/store/*.ts` (Svelte 5 rune-based stores). Toast
  notifications go through `toastStore`.

## Performance budgets (from FMB spec)

| Metric | Target |
| --- | --- |
| First paint of grid (50 rooms) | < 1.5s |
| `GET /api/v1/map` | < 200ms |
| `PATCH /api/v1/rooms/positions` (50 items) | < 500ms |

`teren-qa` enforces these. `teren-db` owns the indexes that make them possible.

# Fix — Seed idempotency + production cleanup

**Date:** 2026-06-30
**Branch:** `fix/seeds/idempotency-and-production-cleanup`
**Scope:** Backend seeds (`backend/seeds/`) + entrypoint (`backend/entrypoint.sh`) + production reconciliation
**Author:** backend rein

## Why this fix exists

Three problems collide in production today:

1. **`backend/entrypoint.sh` runs `./seed` on every container start.**
   The script has zero guards: no environment check, no "already seeded"
   tracking, no idempotency contract. Every Railway redeploy re-executes
   every SQL file in `backend/seeds/`, in order.
2. **`002_seed_bookings.sql` is not idempotent.** Guests are inserted with
   plain `INSERT … RETURNING id`, so each run produces a new row with a
   fresh UUID. Bookings are inserted with plain `INSERT` too — every run
   stacks a new `bookings` row on top of the existing ones, so Maria García
   ends up with six overlapping reservations for room 102 between
   30/06 and 02/07 (see screenshot). Same story for John Doe (USA) and
   Juan Pérez (ESP): one row per deploy.
3. **The database has no UNIQUE constraint** that would protect guests
   from being duplicated. The schema (`001_initial_schema.sql`) defines
   `guests(id, property_id, full_name, phone, nationality, …)` with no
   natural key. The seed could be re-applied safely if the schema
   enforced `(property_id, phone) UNIQUE NULLS DISTINCT`, but it doesn't.

Operational consequences observed in production:

- The `/bookings` endpoint returns 9+ duplicated reservations for the
  same room/guest/date range — every user-action in the UI looks like
  a bug.
- The reservation map (`/map`) flagging system can't dedupe the rows,
  so the visible state is permanently corrupted.
- The owner confirmed only one user is piloting v1.2, so the blast
  radius is contained — but the situation has to be resolved before
  any second user is onboarded.

## What changed (preview, not yet implemented)

### 1. `backend/entrypoint.sh` — guard the seed step

Add a guard before step 2 (`./seed`):

```bash
# Run seeds only in development. Production keeps the data it has
# and never auto-seeds again. Opt in with ENABLE_SEED=true if you
# really need to re-seed (e.g. staging after a wipe).
if [ "${APP_ENV:-development}" = "production" ] && [ "${ENABLE_SEED:-false}" != "true" ]; then
    echo "2. Skipping seeds (APP_ENV=production, ENABLE_SEED!=true)."
else
    echo "2. Running database seeds..."
    ./seed
fi
```

### 2. `backend/seeds/002_seed_bookings.sql` — make it idempotent + non-overlapping

- Guests: replace the plain `INSERT` with `INSERT … ON CONFLICT
  (property_id, phone) DO NOTHING … RETURNING id`. That relies on the
  new UNIQUE index (see point 4).
- Bookings: gate every `INSERT INTO bookings` with a `NOT EXISTS` check
  against an equivalent `(room_id, daterange)` overlap. If a booking
  already exists for that room in that date window, skip the insert and
  log a `RAISE NOTICE`. Uses the `btree_gist` extension so we can put a
  GIST exclusion constraint on `(room_id WITH =, daterange(check_in,
  check_out, '[]') WITH &&)`.
- The `users` insert keeps its `ON CONFLICT DO NOTHING` (already correct).

### 3. `backend/seeds/001_seed_data.sql` / `003_seed_room_types.sql`

Already idempotent (`ON CONFLICT DO NOTHING` / `WHERE NOT EXISTS`) —
just leave them alone. They are not the source of the duplication.

### 4. New migration `backend/migrations/013_guests_unique_phone_and_booking_exclusion.up.sql`

Adds the natural-key protections the seed relies on:

```sql
-- Idempotent: CREATE INDEX IF NOT EXISTS is a no-op when the index exists.
CREATE UNIQUE INDEX IF NOT EXISTS guests_property_phone_unique
    ON guests (property_id, phone)
    WHERE phone IS NOT NULL;

CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_room_no_overlap;
ALTER TABLE bookings
    ADD CONSTRAINT bookings_room_no_overlap
    EXCLUDE USING gist (
        room_id WITH =,
        daterange(check_in::date, check_out::date, '[)') WITH &&
    )
    WHERE (status IN ('confirmed', 'checked_in'));
```

`[)` semantics match the rest of the codebase (check-out day is not
occupied). The `WHERE` clause leaves `cancelled` / `checked_out` rows
free of the constraint so historical data can co-exist.

### 5. New migration `backend/migrations/014_dedupe_guests_and_bookings.up.sql`

Runs **once** on the production database to clean up the duplicates
created by all the past re-execs:

- Backfills a deterministic `email` on duplicated guests (slugified
  `full_name + last-4-of-phone @teren.invalid`) so the seed's
  `ON CONFLICT` path actually triggers — the email is not displayed,
  just a tiebreaker.
- Deletes duplicate `guests` rows keeping the oldest `created_at`
  per `(property_id, phone)` tuple. Re-points `bookings.guest_id` to
  the surviving row inside the same transaction.
- Deletes `bookings` rows that overlap an earlier booking on the same
  room, keeping the earliest one. Logs every row it drops to
  `pg_ctl`-style `RAISE NOTICE` so the operator can audit.
- Has a guard schema at the top (per AGENTS.md rule 1) so it is safe
  to re-run on greenfield databases that already have no duplicates.

The migration is wrapped in a single transaction in Go — same idiom
as `012_align_invoicing_to_v12.sql`.

### 6. `backend/cmd/seed/main.go` — safer production mode

Tighten the existing production guard:

```go
// Treat any environment whose DATABASE_URL resolves to a managed
// cloud DB as production, regardless of -env flag.
isProduction := *envFlag == "production" ||
    strings.Contains(strings.ToLower(dbURL), "railway") ||
    strings.Contains(strings.ToLower(dbURL), "amazonaws") ||
    strings.Contains(strings.ToLower(dbURL), "render.com");

if isProduction && !*forceFlag {
    log.Fatal("refusing to seed in production without --force")
}
```

Adds a new `--report` flag that runs `002_seed_bookings.sql` in dry
mode (counts would-be-affected rows without writing), useful for CI.

## Pre-implementation checklist (operator steps, do these first)

Execute **once** against the Railway production database **before** the
fix ships. The commands are idempotent and safe to re-run.

### Step 1 — snapshot

```bash
# Bash / git-bash.
railway run --service backend -- pg_dump "$DATABASE_URL" \
  > backup_pre_seed_fix_$(date +%Y%m%d_%H%M%S).sql
```

```powershell
# PowerShell alternative — the redirect does not need quoting and the
# date token is built with Get-Date.
$backup = "backup_pre_seed_fix_$(Get-Date -Format 'yyyyMMdd_HHmmss').sql"
railway run --service backend -- pg_dump "$DATABASE_URL" > $backup
```

### Step 2 — stop the bleeding

Set `ENABLE_SEED=false` and `APP_ENV=production` on the Railway
service. After redeploy, the entrypoint will skip `./seed` even when
the container restarts.

### Step 3 — purge duplicate state

Run the contents of `014_dedupe_guests_and_bookings.up.sql` **manually**
via `psql`, do not rely on the auto-migration for this step (we want to
visually confirm the row counts before the runner takes over).

There is a known gotcha that bit us during this fix: the BBDD URL inside
the Railway project (`DATABASE_URL`) points at the **internal** DNS name
(`postgres.railway.internal`) which is only resolvable from inside a
Railway container. If you have `psql` installed locally and paste that
URL into PowerShell, you will see:

```
psql: error: could not translate host name "postgres.railway.internal" to address: Name or service not known
```

…and if `DATABASE_URL` is empty in your shell, psql falls back to your
local PostgreSQL with your Windows username and asks for the **local**
password (`jcdel` in the case of this machine). That is misleading: psql
is asking for the local OS user, not the Railway DB user.

Pick **one** of these approaches.

#### 3a. Local `psql` with the public Railway URL (Windows friendly)

In Railway → backend service → **Variables**, you also get a public URL
(often under a different name like `DATABASE_PUBLIC_URL` or visible in
the **Connect** modal). The public host looks like
`containers-us-west-4.railway.app` and resolves from anywhere.

```powershell
$env:DATABASE_URL = "postgres://USER:PASS@containers-us-west-4.railway.app:PORT/railway?sslmode=require"
psql $env:DATABASE_URL -v ON_ERROR_STOP=1 -f "C:\TEREN\revenue_managment_tools\backend\migrations\014_dedupe_guests_and_bookings.up.sql"
```

This is the variant that worked on this Windows machine on 2026-06-30.

#### 3b. Run `psql` from inside the Railway container (no local install needed)

`railway run` injects `$DATABASE_URL` (the internal one) into the
sub-process, so DNS resolution is automatic. Use this when the public
URL is missing or unstable:

```bash
railway run --service backend -- bash -c 'psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/migrations/014_dedupe_guests_and_bookings.up.sql'
```

#### 3c. From a git-bash terminal on Windows

Git for Windows ships a usable bash environment but does **not** ship
`psql`. Install it (`choco install postgresql` or download the
EnterpriseDB installer), then use 3a or 3b as above. **Do not** rely on
the URL containing the credentials being expanded by your shell — wrap
the assignment in double quotes so `?` and other characters are kept
literal.

Expected output, give or take:

```
NOTICE:  dedupe guests: kept 4, dropped 39 duplicates
NOTICE:  dedupe bookings: dropped 36 overlapping rows
NOTICE:  dedupe bookings: dropped 0 overlapping rows
```

A second run prints `dropped 0 duplicates` and `dropped 0 overlapping rows`,
confirming the migration is fully idempotent.

### Step 4 — verify

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
SELECT count(*) AS guests_total FROM guests;
SELECT count(*) AS bookings_total FROM bookings;
SELECT room_id, check_in, check_out, status
  FROM bookings
 WHERE room_id IN (SELECT id FROM rooms WHERE number IN ('101','102','103'))
 ORDER BY check_in;
"
```

Acceptance criteria:

- `guests_total` ≤ 3 (one row each for Juan, Maria, John).
- `bookings_total` ≤ 3 (one per room, no overlapping dates).
- No two rows share the same `(room_id, daterange(check_in, check_out))`
  for `status IN ('confirmed', 'checked_in')`.

### Step 5 — apply the protective migrations

```bash
railway run --service backend -- ./run-migrations
```

The runner applies `013_guests_unique_phone_and_booking_exclusion`
first (so the dedupe migration can rely on the unique index for its
`ON CONFLICT` clauses) and then `014_dedupe_guests_and_bookings` (no-op
because we already cleaned up manually).

### Step 6 — cut the new branch

Branch: `fix/seeds/idempotency-and-production-cleanup`. PR should
include:

- `backend/seeds/002_seed_bookings.sql` — rewritten for idempotency.
- `backend/entrypoint.sh` — production skip.
- `backend/migrations/013_*` — new UNIQUE + EXCLUDE constraints.
- `backend/migrations/014_*` — dedupe migration (belt-and-braces).
- `backend/cmd/seed/main.go` — tightened production guard + `--report`.
- New unit tests in `backend/cmd/seed/main_test.go` covering the
  guard logic.

CI must include the migration E2E suite in this branch:

```bash
cd backend
go test ./cmd/seed/... -v
go test -tags migration_e2e ./migrations/... -v -count=1
```

## Out of scope (deferred)

- Surfacing duplicate detection in the UI (the map already warns on
  overlapping bookings once the EXCLUDE constraint is in place).
- A "reset demo data" button for the owner — separate product task.
- Backporting the EXCLUDE constraint to staging DBs that already have
  overlapping bookings. They need the dedupe migration first, same
  steps as production.

## Rollback

If 014 misbehaves, the backup from Step 1 is the source of truth:

```bash
railway run --service backend -- pg_dump "$DATABASE_URL" \
  --schema-only --table=bookings \
  > /dev/null  # confirm schema unchanged
psql "$DATABASE_URL" < backup_pre_seed_fix_<TIMESTAMP>.sql
```

Then revert the migration rows:

```sql
DELETE FROM schema_migrations WHERE version IN (13, 14);
```

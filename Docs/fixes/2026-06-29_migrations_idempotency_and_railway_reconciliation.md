# Fix — Idempotency audit + Railway reconciliation

**Date:** 2026-06-29
**Branch:** `fix/migrations/idempotency-and-railway-reconciliation`
**Scope:** Backend migrations + runner + E2E suite
**Author:** backend rein

## Why this fix exists

Two problems collided on production last week:

1. **008_invoice_refunded_status.up.sql** failed in Railway with
   `ERROR: column "status" does not exist (SQLSTATE 42703)`.
   Root cause: two files shared the `006_` version prefix (`006_add_notes.sql`
   and `006_invoicing_schema.up.sql`). The runner deduplicates by numeric
   version, so `006_invoicing_schema.up.sql` was silently skipped and 008
   ran against the legacy `invoices` table.
2. **schema_migrations** had no `filename` column on the legacy deployment.
   The runner's catalog insert `INSERT ... filename` failed before the
   migration was even recorded, so re-running the runner tried to apply 008
   again — same error, infinite loop.

This fix makes the migration suite **idempotent** end-to-end, hardens the
runner so the same class of bug cannot recur, and ships a reconciliation
migration (012) that brings the production database to v1.2 shape regardless
of where it currently stands.

## What changed

### 1. Migrations — guards + `requires:` headers

Every migration that mutates columns now opens with a **guard schema** block
that introspects `information_schema.columns` and logs warnings instead of
failing when prerequisites from earlier migrations are missing. Every ALTER
is either wrapped in `IF NOT EXISTS` / `DROP CONSTRAINT IF EXISTS` or sits
inside a `DO $$ ... $$` that checks `pg_catalog` before mutating.

| File | Change |
| --- | --- |
| `002_schema_corrections.up.sql` | Added `requires:` header. Guard schema at top. `ALTER COLUMN property_id SET NOT NULL` now wrapped in a DO block that only fires when the column has no NULLs and is currently nullable. `bookings.room_id DROP NOT NULL` similarly guarded. `ALTER TYPE room_blocks.reason` wrapped in DO block that skips when `room_blocks` is missing. |
| `004_booking_schema_sync.up.sql` | Added `requires:` header. Guard schema at top. `bookings.room_id DROP NOT NULL` guarded. `UPDATE bookings` uses `original_amount = 0.00 AND total_amount <> 0.00` so re-apply keeps original values. |
| `005_room_status_cleaning.up.sql` | Added `requires:` header. Guard schema at top. Body otherwise unchanged (already idempotent via DO block). |
| `008_invoice_refunded_status.up.sql` | `requires:` header moved to file header (was buried at line 6). Guard schema added. All DDL was already idempotent. |
| `011_invoice_notes_column.sql` | `requires:` header verified at top. |
| `012_align_invoicing_to_v12.sql` | New file. Reconciles legacy prod to v1.2 — see section 3. |

### 2. Runner hardening (`backend/cmd/migrations/main.go`)

- **`--dry-run` flag.** Executes each pending migration inside a transaction
  and rolls back unconditionally. Exit code 0 means the migration would have
  succeeded against the current schema; 1 means a real apply would have failed.
- **`requires:` header parsing restricted to the first 20 lines** of each
  file. A `-- requires:` comment buried in the middle of a SQL script can
  no longer be misinterpreted as a precondition.
- **Filename vs catalog collision is fatal.** `assertNoVersionCollisions`
  now compares the recorded `filename` against the file on disk for every
  version. If the runner finds `version=006` recorded with filename
  `006_add_notes.sql` but the directory only ships `006_invoicing_schema.up.sql`,
  the runner aborts with `Migration catalog conflict (filename/version mismatch)`
  before applying anything. This is the same check that would have prevented
  the 008 failure.
- **`warnMissingRequires`.** Logs a non-fatal `WARNING: ... has no
  'requires:' header` for any migration that does not declare its
  preconditions. CI can promote this to a hard failure by setting
  `MIGRATIONS_STRICT_REQUIRES=1` (future work).

### 3. Migration 012 — reconcile legacy production

`012_align_invoicing_to_v12.sql` brings any database — greenfield, legacy
catalog, or legacy invoicing shape — to the v1.2 invoicing schema. It is
idempotent end-to-end:

1. Backups the legacy `invoices` table (if present) to
   `legacy_invoices_backup` then drops it. **No production data is
   preserved** by design: the legacy table carried MVP test data only and
   the owner confirmed a single user is still piloting v1.2 features.
2. Adds `bookings.force_override` if missing (006 expectation).
3. Recreates the full v1.2 invoicing schema (`invoices`, `invoice_sequences`,
   `invoice_line_items`, `payments`, `refund_batches`, `idempotency_keys`,
   functions and triggers). Every object uses `IF NOT EXISTS` / `OR REPLACE`
   so a re-apply is a no-op.
4. Bootstraps `schema_migrations.filename` so the runner can detect
   version-prefix collisions from now on.

The E2E suite verifies 012 against three starting shapes:
- Empty database with greenfield catalog.
- Database with the legacy `schema_migrations(version, applied_at)` catalog.
- Database with the legacy `invoices` table (with `payment_status`) and
  `version=6` already recorded in the catalog — the exact production state
  at the moment 008 failed.

### 4. E2E suite (`backend/migrations/migrations_e2e_test.go`)

Five tests, all running against an embedded PostgreSQL 16:

| Test | Scenario |
| --- | --- |
| `TestMigrationsApplyGreenfield` | Empty DB, greenfield catalog, expect v1.2 shape |
| `TestMigrationsApplyLegacyCatalog` | Legacy catalog (no `filename`), expect v1.2 shape and `filename` column present |
| `TestMigrationsApplyLegacyInvoicingTable` | Legacy `invoices` table + `version=6` recorded, expect v1.2 shape (012 must reconcile) |
| `TestMigrationsApplyIdempotentlyPerFile` | Apply each migration twice in isolation, expect no errors and v1.2 shape |
| `TestMigrationsApplyIdempotentlyBatch` | Apply the whole suite twice in order, expect no errors and v1.2 shape |

Run them locally:

```bash
cd backend
go test -tags migration_e2e ./migrations/... -v -count=1
```

The tests are gated by `-tags migration_e2e` and skip themselves when
`DATABASE_URL` is set so they never fight the dev database.

## How to reconcile Railway production

The fix is self-healing for greenfield deployments — Railway's
`entrypoint.sh` calls `./run-migrations` on every container start, and the
runner now reconciles everything in one pass. For the existing production
database, follow the steps below **exactly once**.

### Pre-flight

Confirm the production state matches what 012 expects to reconcile:

```bash
# Connect via Railway's psql (one-shot)
railway run --service backend -- psql "$DATABASE_URL" -c "\d schema_migrations"
railway run --service backend -- psql "$DATABASE_URL" -c "\d invoices"
railway run --service backend -- psql "$DATABASE_URL" -c "SELECT version, filename FROM schema_migrations ORDER BY version;"
```

You should see the legacy catalog (no `filename` column), the legacy
`invoices` table with `payment_status`, and at least the row
`version=6, filename=NULL` recorded.

### Step 1 — back up the catalog

```bash
railway run --service backend -- pg_dump "$DATABASE_URL" \
  --schema-only --no-owner --table=schema_migrations \
  > backup_schema_migrations_$(date +%Y%m%d_%H%M%S).sql
```

### Step 2 — align `schema_migrations.filename`

The runner's catalog needs the `filename` column before 012 can record its
own entry. Run this once by hand:

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS filename TEXT;

-- Backfill filenames for migrations that the runner has shipped since the
-- runner became filename-aware. Older rows stay NULL — 012 will backfill
-- itself in its INSERT step.
UPDATE schema_migrations SET filename = '001_initial_schema.sql'        WHERE version = 1 AND filename IS NULL;
UPDATE schema_migrations SET filename = '002_schema_corrections.up.sql' WHERE version = 2 AND filename IS NULL;
UPDATE schema_migrations SET filename = '004_booking_schema_sync.up.sql' WHERE version = 4 AND filename IS NULL;
UPDATE schema_migrations SET filename = '005_room_status_cleaning.up.sql' WHERE version = 5 AND filename IS NULL;
UPDATE schema_migrations SET filename = '006_invoicing_schema.up.sql'    WHERE version = 6 AND filename IS NULL;
UPDATE schema_migrations SET filename = '008_invoice_refunded_status.up.sql' WHERE version = 8 AND filename IS NULL;
UPDATE schema_migrations SET filename = '009_invoice_number_sync.up.sql' WHERE version = 9 AND filename IS NULL;
UPDATE schema_migrations SET filename = '010_invoice_refunded_status_fix.up.sql' WHERE version = 10 AND filename IS NULL;
UPDATE schema_migrations SET filename = '011_invoice_notes_column.sql'   WHERE version = 11 AND filename IS NULL;
"
```

If a different file actually shipped under a given version (e.g. the legacy
deployment recorded `006_add_notes.sql` for version 6), rename the row to
match the file you want 012 to skip:

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
UPDATE schema_migrations SET filename = '006_invoicing_schema.up.sql'
 WHERE version = 6 AND filename = '006_add_notes.sql';
"
```

### Step 3 — dry-run 012 against production

The runner ships with `--dry-run`. Run it against Railway without committing
anything:

```bash
railway run --service backend --env DATABASE_URL="$DATABASE_URL" -- \
  ./run-migrations --dry-run
```

You should see `[dry-run] Would apply migration 012 ...` and exit 0. If it
exits non-zero, inspect the logs — the runner now reports the exact
filename/version mismatch that aborted.

### Step 4 — apply 012 for real

```bash
railway run --service backend -- ./run-migrations
```

The runner will:

1. Skip versions 1..11 (already in `schema_migrations`).
2. Discover version 12 as pending.
3. Verify its `requires:` precondition (`schema_migrations.version`).
4. Apply 012 inside a transaction. 012 backfills `filename` for the older
   rows in its INSERT step, so the catalog ends in a fully consistent state.

### Step 5 — verify

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
SELECT version, filename FROM schema_migrations ORDER BY version;
\d invoices
SELECT COUNT(*) AS needs_review_count FROM invoices WHERE needs_review = TRUE;
"
```

Expect:

- Rows for versions 1..12, every `filename` populated.
- `invoices` with `status` (`active|void|refunded`), `needs_review`, `notes`.
- `needs_review_count = 0` for any invoice whose refunds sum to within
  `total ± 0.01`.

### Step 6 — confirm no orphans

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
SELECT COUNT(*) FROM legacy_invoices_backup;
SELECT COUNT(*) FROM payments WHERE invalidated_at IS NOT NULL;
"
```

`legacy_invoices_backup` is preserved for forensic purposes. Drop it
manually after the owner signs off:

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
DROP TABLE IF EXISTS legacy_invoices_backup;
"
```

## Validation matrix

| Check | Command | Pass criteria |
| --- | --- | --- |
| Lint | `go vet ./...` | exit 0 |
| Build | `go build ./...` | exit 0 |
| Unit tests | `go test ./... -count=1` | all packages `ok` |
| Runner parser tests | `go test ./cmd/migrations/... -v` | 7/7 PASS |
| E2E migrations (embedded PG 16) | `go test -tags migration_e2e ./migrations/... -v` | 5/5 PASS |
| Dry-run against Railway | `railway run -- ./run-migrations --dry-run` | exit 0, all 10 migrations marked "would apply" or "already applied" |
| Real apply against Railway | `railway run -- ./run-migrations` | exit 0, `schema_migrations` row for version 12 |

Local validation runs (2026-06-29):

```
$ go vet ./...
(no output)

$ go build ./cmd/migrations/...
(no output)

$ go test ./cmd/migrations/... -v
=== RUN   TestParseRequirements_NoHeader      --- PASS (0.00s)
=== RUN   TestParseRequirements_SingleColumn  --- PASS (0.00s)
=== RUN   TestParseRequirements_MultipleColumns --- PASS (0.00s)
=== RUN   TestParseRequirements_Malformed     --- PASS (0.00s)
=== RUN   TestDiscoverMigrations_DropsDownSuffix --- PASS (0.00s)
=== RUN   TestAssertNoVersionCollisions_OK    --- PASS (0.00s)
=== RUN   TestAssertNoVersionCollisions_Duplicate --- PASS (0.00s)
PASS

$ go test ./...
ok  cmd/migrations      0.752s
ok  internal/api        3.424s
ok  internal/invoicing  2.526s
ok  internal/repository 3.042s
ok  internal/service    6.041s
ok  pkg/pdfgen          1.944s

$ go test -tags migration_e2e ./migrations/... -v -count=1
=== RUN   TestMigrationsApplyGreenfield              --- PASS (16.97s)
=== RUN   TestMigrationsApplyLegacyCatalog           --- PASS (15.92s)
=== RUN   TestMigrationsApplyLegacyInvoicingTable    --- PASS (15.29s)
=== RUN   TestMigrationsApplyIdempotentlyPerFile     --- PASS (15.95s)
=== RUN   TestMigrationsApplyIdempotentlyBatch       --- PASS (16.22s)
PASS
```

## Files touched

| File | Why |
| --- | --- |
| `backend/migrations/002_schema_corrections.up.sql` | Idempotency + `requires:` |
| `backend/migrations/004_booking_schema_sync.up.sql` | Idempotency + `requires:` |
| `backend/migrations/005_room_status_cleaning.up.sql` | Idempotency + `requires:` |
| `backend/migrations/008_invoice_refunded_status.up.sql` | Move `requires:` to header + guard schema |
| `backend/migrations/011_invoice_notes_column.sql` | Verify `requires:` header |
| `backend/migrations/012_align_invoicing_to_v12.sql` | New — reconcile legacy production |
| `backend/cmd/migrations/main.go` | Header scope, filename collision fatal, warning on missing `requires:` |
| `backend/migrations/migrations_e2e_test.go` | New tests + refactor |

## Rollback

If 012 misbehaves on production, the immediate rollback is:

```bash
railway run --service backend -- psql "$DATABASE_URL" -c "
DELETE FROM schema_migrations WHERE version = 12;
-- legacy_invoices_backup is preserved; restore manually if needed:
-- DROP TABLE invoices CASCADE;
-- ALTER TABLE legacy_invoices_backup RENAME TO invoices;
"
```

Then redeploy without the runner's catalog row for 12 and the next run
will re-attempt 012 against the same starting state.

## Follow-ups (out of scope here)

- Promote `warnMissingRequires` to a hard failure under CI by setting
  `MIGRATIONS_STRICT_REQUIRES=1`. Tracked separately.
- Add a `pg_dump --schema-only` golden file test that fails the PR pipeline
  when any migration produces an unexpected diff. Tracked separately.
- `006_add_notes.sql` from the legacy deployment is still in some
  developers' working copies; audit `git ls-files | grep 006_` and remove
  duplicates.
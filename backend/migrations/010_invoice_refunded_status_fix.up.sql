-- 010_invoice_refunded_status_fix.up.sql
-- Invoicing v1.2 — Refund 1:1 + Refunded status lifecycle (idempotent fix)
-- Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.2.md
-- Ratified: R-07 (refund 1:1) + R-08 (refunded status) + R-09 (UX details)
--
-- Context:
--   008_invoice_refunded_status.up.sql failed in production with:
--       ERROR: column "status" does not exist (SQLSTATE 42703)
--   The root cause was twofold:
--     1. Two files shared the "006_" version prefix (006_add_notes.sql and
--        006_invoicing_schema.up.sql). The runner deduplicates by version,
--        so partial application prior to 008 left the schema inconsistent.
--     2. 008 referenced invoices.status, invoices.needs_review,
--        payments.invalidated_*, refund_batches, and
--        trg_invoice_status_update without idempotent guards.
--   This migration is fully idempotent and safe to apply repeatedly. It uses
--   dynamic DO blocks to introspect the catalog and create or repair only
--   what is missing — regardless of which 006 (or none) is present.
--
-- Summary of changes (idempotent):
--   1. Ensure `invoices` exists with a `status TEXT` column (active|void|refunded).
--   2. Ensure the `status` CHECK constraint includes 'refunded'.
--   3. Ensure `invoices.needs_review` exists.
--   4. Ensure `payments.invalidated_*` columns exist (R-07).
--   5. Ensure the `refund_batches` table exists.
--   6. Ensure the `trg_invoice_status_update` function and trigger exist.
--   7. Re-evaluate invoice lifecycle and flag over-refunded rows (idempotent).
--   8. Invalidate the two legacy rows from MVP testing (idempotent).
--
-- Bootstrap of the runner's own catalog (coupled to this migration):
--   The migration runner uses `schema_migrations.filename` to detect
--   version-prefix collisions (e.g. two files sharing the "006_" prefix).
--   This file was created at the same time as that detection logic, so it
--   also bootstraps the column on existing deployments where the runner was
--   bootstrapped by an older `CREATE TABLE` statement that pre-dates it.

-- =============================================================================
-- 0. Bootstrap: ensure the runner's catalog tracks the migration filename
--    Old deployments booted the runner with a `schema_migrations(version,
--    applied_at)` table that has no `filename` column. Without this guard,
--    the runner's INSERT-with-filename would fail at apply time.
-- =============================================================================
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS filename TEXT;

COMMENT ON COLUMN schema_migrations.filename IS
    'Filename of the migration file associated with this version. Used by the runner to detect version-prefix collisions (e.g. 006_add_notes.sql and 006_invoicing_schema.up.sql sharing "006_").';

-- =============================================================================
-- 1. Ensure `invoices` table exists with a `status` column
--    If the table was never created (e.g. 006_invoicing_schema never applied),
--    build the minimum schema required by later sections. If it already
--    exists, leave the structure as-is and only patch missing columns below.
-- =============================================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables
                   WHERE table_schema = 'public' AND table_name = 'invoices') THEN
        EXECUTE $SQL$
            CREATE TABLE invoices (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
                booking_id UUID NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE RESTRICT,
                invoice_number VARCHAR(30) NOT NULL,
                subtotal NUMERIC(12,2) NOT NULL CHECK (subtotal >= 0),
                tax_amount NUMERIC(12,2) NOT NULL CHECK (tax_amount >= 0),
                total NUMERIC(12,2) NOT NULL CHECK (total >= 0),
                status TEXT NOT NULL DEFAULT 'active',
                issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                paid_at TIMESTAMPTZ,
                voided_at TIMESTAMPTZ,
                voided_by UUID REFERENCES users(id),
                void_reason TEXT,
                created_by UUID NOT NULL REFERENCES users(id),
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                CONSTRAINT invoices_number_per_property_unique UNIQUE (property_id, invoice_number)
            )
        $SQL$;
    END IF;
END$$;

-- 1.b) Ensure `status` column exists (no-op if already present)
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS status TEXT;

-- 1.c) Backfill NOT NULL safely: set default, replace NULLs, then enforce NOT NULL.
--      Each ALTER is wrapped so that re-applying on a fully-shaped table is a no-op.
DO $$
BEGIN
    -- Default value for any future inserts/backfills
    BEGIN
        EXECUTE 'ALTER TABLE invoices ALTER COLUMN status SET DEFAULT ''active''';
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
    -- Replace any existing NULLs with 'active' before tightening the constraint
    EXECUTE 'UPDATE invoices SET status = ''active'' WHERE status IS NULL';
    -- Enforce NOT NULL when the table shape allows it
    BEGIN
        EXECUTE 'ALTER TABLE invoices ALTER COLUMN status SET NOT NULL';
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
END$$;

-- =============================================================================
-- 2. Ensure `invoices.status` CHECK constraint includes 'refunded'
--    Drop the existing named constraint if present and recreate it with the
--    extended domain. Done via DO block because ALTER TABLE ... ADD
--    CONSTRAINT is not idempotent on its own.
-- =============================================================================
DO $$
DECLARE v_has_check BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'invoices_status_check'
    ) INTO v_has_check;

    IF v_has_check THEN
        EXECUTE 'ALTER TABLE invoices DROP CONSTRAINT invoices_status_check';
    END IF;

    EXECUTE $SQL$
        ALTER TABLE invoices
            ADD CONSTRAINT invoices_status_check
            CHECK (status IN ('active','void','refunded'))
    $SQL$;
END$$;

-- =============================================================================
-- 3. Ensure `invoices.needs_review` data-integrity flag (R-08)
-- =============================================================================
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS needs_review BOOLEAN
    NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_invoices_needs_review
    ON invoices(property_id) WHERE needs_review = TRUE;

COMMENT ON COLUMN invoices.needs_review IS
    'R-08: TRUE = data integrity issue (e.g. legacy over-refund). Excluded from revenue/tax reports until owner resolves manually.';

-- =============================================================================
-- 4. Ensure `payments.invalidated_*` columns exist (R-07)
--    Used by POST /refund-all to retire bad rows without losing audit trail.
-- =============================================================================
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_by UUID REFERENCES users(id);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_payments_invalidated
    ON payments(invoice_id) WHERE invalidated_at IS NOT NULL;

COMMENT ON COLUMN payments.invalidated_at IS
    'R-07: when set, the row is excluded from total_paid / total_refunded / effective_status computations.';

-- =============================================================================
-- 5. Ensure `refund_batches` audit table exists (R-07)
-- =============================================================================
CREATE TABLE IF NOT EXISTS refund_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    initiated_by UUID NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    payment_ids UUID[] NOT NULL,
    total_refunded NUMERIC(12,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refund_batches_invoice
    ON refund_batches(invoice_id);

CREATE INDEX IF NOT EXISTS idx_refund_batches_property_date
    ON refund_batches(property_id, created_at DESC);

COMMENT ON TABLE refund_batches IS
    'R-07: audit trail for atomic batch refunds (POST /refund-all). One row per batch even if N refunds were generated.';

-- =============================================================================
-- 6. Ensure `trg_invoice_status_update` function and trigger exist (R-08)
--    Recreates the function (idempotent) and reinstalls the trigger. Only
--    auto-promotes active → refunded when total_refunded >= total. Terminal
--    states (void, refunded) are never overwritten.
-- =============================================================================
CREATE OR REPLACE FUNCTION trg_invoice_status_update()
RETURNS TRIGGER AS $$
DECLARE
    v_invoice_id UUID;
    v_total NUMERIC(12,2);
    v_paid NUMERIC(12,2);
    v_refunded NUMERIC(12,2);
    v_lifecycle TEXT;
BEGIN
    -- Resolve the invoice id regardless of INSERT/UPDATE/DELETE
    v_invoice_id := COALESCE(NEW.invoice_id, OLD.invoice_id);

    -- Aggregate active (non-invalidated) payments for the invoice
    SELECT i.total,
           COALESCE(SUM(CASE WHEN p.amount > 0 THEN p.amount END), 0),
           COALESCE(ABS(SUM(CASE WHEN p.amount < 0 THEN p.amount END)), 0)
      INTO v_total, v_paid, v_refunded
      FROM invoices i
      LEFT JOIN payments p
        ON p.invoice_id = i.id
       AND p.invalidated_at IS NULL
     WHERE i.id = v_invoice_id
     GROUP BY i.total;

    -- Read current lifecycle to honour terminal states
    SELECT status INTO v_lifecycle FROM invoices WHERE id = v_invoice_id;

    -- Auto-promote only when the lifecycle is 'active' and the invoice is fully refunded
    IF v_lifecycle = 'active' AND COALESCE(v_total, 0) > 0 THEN
        IF v_refunded >= v_total THEN
            UPDATE invoices SET status = 'refunded' WHERE id = v_invoice_id;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_status_update ON payments;
CREATE TRIGGER invoice_status_update
    AFTER INSERT OR UPDATE OR DELETE ON payments
    FOR EACH ROW EXECUTE FUNCTION trg_invoice_status_update();

COMMENT ON FUNCTION trg_invoice_status_update() IS
    'R-08: keeps invoices.status in sync with payment activity. Only auto-promotes active → refunded when total_refunded >= total. Terminal states (void, refunded) are never auto-overwritten.';

-- =============================================================================
-- 7. Data migration — idempotent
-- =============================================================================

-- 7.1 Retire the two legacy rows from MVP testing.
--      WHERE clause plus COALESCE guards make this safe to apply repeatedly.
UPDATE payments
   SET invalidated_at = COALESCE(invalidated_at, NOW()),
       invalidated_by = COALESCE(invalidated_by, (SELECT id FROM users WHERE role = 'owner' LIMIT 1)),
       invalidated_reason = COALESCE(invalidated_reason, 'INVALIDATED - manual split required')
 WHERE id IN (
     'a94e74eb-e039-4876-9b7c-845d35e48ffc',
     '0ac15330-fdfd-43a5-9ecf-5505bd6c0d75'
 )
   AND invalidated_at IS NULL;

-- 7.2 Promote active invoices to 'refunded' when total_refunded >= total.
--      Idempotent: the WHERE clause skips rows already in 'refunded'.
UPDATE invoices i
   SET status = 'refunded'
 WHERE i.status = 'active'
   AND i.total > 0
   AND COALESCE((
       SELECT ABS(SUM(p.amount))
         FROM payments p
        WHERE p.invoice_id = i.id
          AND p.amount < 0
          AND p.invalidated_at IS NULL
   ), 0) >= i.total;

-- 7.3 Flag over-refunded invoices for manual review (R-08, BR-INV-011).
--      Idempotent: only flips rows that are currently needs_review = FALSE.
UPDATE invoices i
   SET needs_review = TRUE
 WHERE (i.status IN ('active','refunded') OR i.status IS NULL)
   AND COALESCE((
       SELECT ABS(SUM(p.amount))
         FROM payments p
        WHERE p.invoice_id = i.id
          AND p.amount < 0
          AND p.invalidated_at IS NULL
   ), 0) > i.total + 0.01
   AND i.needs_review = FALSE;

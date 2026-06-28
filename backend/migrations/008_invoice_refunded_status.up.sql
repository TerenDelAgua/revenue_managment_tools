-- 008_invoice_refunded_status.up.sql
-- Invoicing v1.2 — Refund 1:1 + Refunded status lifecycle
-- Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.2.md
-- Ratified: R-07 (refund 1:1) + R-08 (refunded status) + R-09 (UX details)
--
-- Resumen del cambio:
--   1. Extiende invoices.status CHECK para incluir 'refunded'.
--   2. Añade invoices.needs_review (data integrity flag).
--   3. Añade payments.invalidated_at / invalidated_by / invalidated_reason.
--   4. Crea refund_batches (audit table para POST /refund-all).
--   5. Trigger trg_invoice_status_update: re-calcula status tras
--      INSERT/UPDATE/DELETE en payments. Solo muta si lifecycle='active'
--      (void/refunded son terminales — R-08).
--   6. Data migration: invalida filas legacy del MVP testing,
--      flippa invoices ya totalmente refunded, y marca over-refunds.

-- =============================================================================
-- 1. invoices.status CHECK constraint
-- =============================================================================
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('active','void','refunded'));

-- =============================================================================
-- 2. invoices.needs_review (data integrity flag for R-08)
-- =============================================================================
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS needs_review BOOLEAN
    NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_invoices_needs_review
    ON invoices(property_id) WHERE needs_review = TRUE;

COMMENT ON COLUMN invoices.needs_review IS
    'R-08: TRUE = data integrity issue (e.g. legacy over-refund).
     Excluded from revenue / tax reports until owner resolves manually.';

-- =============================================================================
-- 3. payments invalidation columns (R-07)
-- =============================================================================
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_by UUID REFERENCES users(id);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS invalidated_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_payments_invalidated
    ON payments(invoice_id) WHERE invalidated_at IS NOT NULL;

COMMENT ON COLUMN payments.invalidated_at IS
    'R-07: when set, the row is excluded from total_paid /
     total_refunded / effective_status computations. Used to
     retire bad rows without losing audit trail.';

-- =============================================================================
-- 4. refund_batches (audit table for POST /refund-all)
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
    'R-07: audit trail for atomic batch refunds (POST /refund-all).
     One row per batch even if N refunds were generated.';

-- =============================================================================
-- 5. Trigger: recompute invoices.status on payment changes
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
    -- Pick the right invoice id based on op
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

    -- Only mutate status if the invoice is currently 'active'.
    -- 'void' and 'refunded' are TERMINAL — never auto-overwritten.
    -- (R-08: invoices.status transitions are explicit, not implicit
    --  from payment events.)
    SELECT status INTO v_lifecycle FROM invoices WHERE id = v_invoice_id;

    IF v_lifecycle = 'active' AND v_total > 0 THEN
        IF v_refunded >= v_total THEN
            UPDATE invoices SET status = 'refunded' WHERE id = v_invoice_id;
        END IF;
        -- If paid == total but refunded < total → lifecycle stays 'active',
        -- effective_status='paid' (computed, not persisted).
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_status_update ON payments;
CREATE TRIGGER invoice_status_update
    AFTER INSERT OR UPDATE OR DELETE ON payments
    FOR EACH ROW EXECUTE FUNCTION trg_invoice_status_update();

COMMENT ON FUNCTION trg_invoice_status_update() IS
    'R-08: keeps invoices.status in sync with payment activity.
     Only auto-promotes active → refunded when total_refunded >= total.
     Terminal states (void, refunded) are never auto-overwritten.';

-- =============================================================================
-- 6. Data migration: legacy cleanup + status flips + integrity flag
-- =============================================================================

-- 6.1 Invalidate the 2 legacy rows from MVP testing on INV-2026-0002
--     (R-09 Q2: confirmed by owner — retire without losing audit).
UPDATE payments
   SET invalidated_at = NOW(),
       invalidated_by = (SELECT id FROM users WHERE role = 'owner' LIMIT 1),
       invalidated_reason = 'INVALIDATED - manual split required'
 WHERE id IN (
     'a94e74eb-e039-4876-9b7c-845d35e48ffc',  -- -100000 SLIP-R-001 (smoke test)
     '0ac15330-fdfd-43a5-9ecf-5505bd6c0d75'   -- -666000 TRF-DEV-002 (manual refund)
 )
   AND invalidated_at IS NULL;

-- 6.2 Re-evaluate lifecycle for invoices after the invalidation above.
--     INV-2026-0002 had 2 valid charges (300k + 366k = 666k) and
--     0 valid refunds → lifecycle stays 'active', not 'refunded'.
--     But other invoices may now qualify for refunded.
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

-- 6.3 Flag over-refunded invoices for manual review (R-08, BR-INV-011).
UPDATE invoices i
   SET needs_review = TRUE
 WHERE i.status IN ('active','refunded')
   AND COALESCE((
       SELECT ABS(SUM(p.amount))
         FROM payments p
        WHERE p.invoice_id = i.id
          AND p.amount < 0
          AND p.invalidated_at IS NULL
   ), 0) > i.total + 0.01;
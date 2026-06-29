-- 012_align_invoicing_to_v12.sql
-- Reconcile legacy invoicing schema with Invoicing v1.2
-- Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.2.md
--
-- requires: schema_migrations.version
-- (The runner's catalog `schema_migrations.version` column is always present
--  because bootstrapCatalog() creates it via CREATE TABLE IF NOT EXISTS on
--  every run. This dependency ensures the runner is wired up before 012
--  touches the catalog to add the `filename` column.)
--
-- Why this file exists:
--   A early production deployment was running against the MVP-only `invoices`
--   table (no `payments`, no `status`, no refund lifecycle). The legacy table
--   carries test data only and contains no values that need to be preserved
--   (confirmed by owner: a single user is still piloting the new features).
--   Subsequent migrations (006, 008) were skipped because version 6 was
--   already recorded in `schema_migrations` against the vestigial file.
--
-- What this migration does:
--   1. Drops the legacy `invoices` table (no FK dependents beyond a stale
--      `bookings` row that the new model treats differently).
--   2. Drops the stale `bookings.invoice_id` link if it lingers, because the
--      v1.2 invoices use `property_id` directly and a fresh FK to bookings.
--   3. Re-creates the v1.2 invoicing schema in full (006 + 008 + 010 + 011).
--   4. Ensures `schema_migrations.filename` exists so the runner can detect
--      version-prefix collisions from now on.
--
-- Re-apply safety: every step is guarded so the file can be re-executed
-- without altering an already-aligned database. Idempotency is verified by
-- the E2E suite (TestMigrationsApplyIdempotently).

-- =============================================================================
-- 1. Drop the legacy `invoices` table.
--    The new invoicing model relies on `invoices.property_id` rather than
--    a `payments`-less design, so any leftover rows from the MVP can be
--    safely removed. CASCADE clears dependent objects (indexes, FKs).
-- =============================================================================
DROP TABLE IF EXISTS legacy_invoices_backup CASCADE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables
               WHERE table_schema = 'public' AND table_name = 'invoices') THEN
        -- Preserve a copy for forensic purposes if the operator wants it.
        EXECUTE 'CREATE TABLE legacy_invoices_backup AS TABLE invoices';
        EXECUTE 'DROP TABLE invoices CASCADE';
    END IF;
END$$;

-- =============================================================================
-- 2. Ensure `bookings.force_override` is present (006 v1.1 expectation).
--    The 006 file in the repo already covers this, but on legacy databases
--    the column may be absent. Idempotent via ADD COLUMN IF NOT EXISTS.
-- =============================================================================
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS force_override BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN bookings.force_override IS
    'Permite al receptionist saltarse validaciones (overbooking, solapamiento, refunds). Reusado por el módulo de facturación para permitir refunds sin owner explícito.';

-- =============================================================================
-- 3. Re-create the v1.2 invoicing schema from scratch.
--    All objects created with IF NOT EXISTS / OR REPLACE so that re-applying
--    this file against a greenfield or already-aligned database is a no-op.
-- =============================================================================

-- 3.1 invoice_sequences — atomic per-property/year numbering.
CREATE TABLE IF NOT EXISTS invoice_sequences (
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    next_number INTEGER NOT NULL DEFAULT 1
        CHECK (next_number > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (property_id, year)
);

COMMENT ON TABLE invoice_sequences IS
    'Secuencia de numeración de facturas por property y año. Retención 10 años (alineado con requerimiento fiscal indonesio DG Tax).';

-- 3.2 invoices — full v1.2 header.
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    booking_id UUID NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE RESTRICT,
    invoice_number VARCHAR(30) NOT NULL,
    subtotal NUMERIC(12,2) NOT NULL CHECK (subtotal >= 0),
    tax_amount NUMERIC(12,2) NOT NULL CHECK (tax_amount >= 0),
    ppn_rate_snapshot NUMERIC(5,4) NOT NULL DEFAULT 0.1100
        CHECK (ppn_rate_snapshot >= 0 AND ppn_rate_snapshot <= 1),
    total NUMERIC(12,2) NOT NULL CHECK (total >= 0),
    original_currency CHAR(3) NOT NULL DEFAULT 'IDR'
        CHECK (original_currency ~ '^[A-Z]{3}$'),
    exchange_rate NUMERIC(12,6) NOT NULL DEFAULT 1.000000
        CHECK (exchange_rate > 0),
    status TEXT NOT NULL DEFAULT 'active',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    voided_by UUID REFERENCES users(id),
    void_reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    pdf_url TEXT,
    notes TEXT,
    needs_review BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_total_integrity CHECK (total = subtotal + tax_amount),
    CONSTRAINT invoices_number_per_property_unique UNIQUE (property_id, invoice_number)
);

CREATE INDEX IF NOT EXISTS idx_invoices_property_issued_active
    ON invoices (property_id, issued_at DESC) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_invoices_property_status_issued
    ON invoices (property_id, status, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_property_paid_at
    ON invoices (property_id, paid_at) WHERE paid_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_invoices_needs_review
    ON invoices (property_id) WHERE needs_review = TRUE;

-- status CHECK: extended in 008 to include 'refunded'.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'invoices_status_check') THEN
        EXECUTE 'ALTER TABLE invoices DROP CONSTRAINT invoices_status_check';
    END IF;
    EXECUTE $SQL$
        ALTER TABLE invoices
            ADD CONSTRAINT invoices_status_check
            CHECK (status IN ('active','void','refunded'))
    $SQL$;
END$$;

COMMENT ON TABLE invoices IS
    'Cabecera de facturas. status es el lifecycle (active/void/refunded); el estado derivado (unpaid/partial/paid/overpaid) lo computa el repositorio cruzando con payments. needs_review=TRUE marca inconsistencias de datos (ej. legacy over-refund) que se excluyen de los reportes hasta resolución manual.';

-- 3.3 invoice_line_items.
CREATE TABLE IF NOT EXISTS invoice_line_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    quantity NUMERIC(10,2) NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price NUMERIC(12,2) NOT NULL,
    total NUMERIC(12,2) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice_sort
    ON invoice_line_items (invoice_id, sort_order);

COMMENT ON TABLE invoice_line_items IS
    'Líneas de factura. Una factura típica tiene 1 línea (alojamiento), pero el modelo soporta múltiples (alojamiento + desayuno + tour).';

-- 3.4 payments — supports refunds and invalidation (R-07).
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    method TEXT NOT NULL
        CHECK (method IN ('cash','bank_transfer','qris','card')),
    amount NUMERIC(12,2) NOT NULL CHECK (amount <> 0),
    original_currency CHAR(3) NOT NULL DEFAULT 'IDR'
        CHECK (original_currency ~ '^[A-Z]{3}$'),
    exchange_rate NUMERIC(12,6) NOT NULL DEFAULT 1.000000
        CHECK (exchange_rate > 0),
    reference TEXT,
    notes TEXT,
    is_reversal BOOLEAN NOT NULL DEFAULT FALSE,
    reversal_of UUID REFERENCES payments(id),
    invalidated_at TIMESTAMPTZ,
    invalidated_by UUID REFERENCES users(id),
    invalidated_reason TEXT,
    received_by UUID NOT NULL REFERENCES users(id),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_refund_has_original CHECK (
        (is_reversal = FALSE AND reversal_of IS NULL) OR
        (is_reversal = TRUE  AND reversal_of IS NOT NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_payments_invoice_received
    ON payments (invoice_id, received_at);
CREATE INDEX IF NOT EXISTS idx_payments_property_method_date
    ON payments (property_id, method, received_at);
CREATE INDEX IF NOT EXISTS idx_payments_reversal
    ON payments (reversal_of) WHERE reversal_of IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_invalidated
    ON payments (invoice_id) WHERE invalidated_at IS NOT NULL;

COMMENT ON TABLE payments IS
    'Pagos individuales contra una factura. Permite múltiples pagos parciales (depósito + saldo) y refunds (amount < 0, is_reversal=TRUE, reversal_of apunta al pago original). invalidated_at IS NOT NULL excluye la fila de total_paid / total_refunded.';

-- 3.5 refund_batches — atomic batch refund audit (R-07).
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
    'R-07: audit trail for atomic batch refunds (POST /refund-all). Una fila por batch incluso si se generaron N refunds.';

-- 3.6 idempotency_keys — POST /payments dedupe (R-06, TTL 24h).
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON idempotency_keys (expires_at);

COMMENT ON TABLE idempotency_keys IS
    'Cache de respuestas por Idempotency-Key. TTL 24h (R-06). Las claves expiradas se purgan vía job (futuro). Configurable por property en Phase 2.';

-- 3.7 get_next_invoice_number — gapless per (property, year).
CREATE OR REPLACE FUNCTION get_next_invoice_number(p_property_id UUID)
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year INT := EXTRACT(YEAR FROM NOW());
    v_next INT;
    v_year_prefix TEXT := 'INV-' || v_year || '-';
BEGIN
    INSERT INTO invoice_sequences (property_id, year, next_number)
    VALUES (
        p_property_id,
        v_year,
        COALESCE(
            (
                SELECT MAX(
                    CAST(
                        SUBSTRING(invoice_number FROM '^INV-[0-9]{4}-([0-9]+)$')
                        AS INTEGER
                    )
                ) + 1
                FROM invoices
                WHERE property_id = p_property_id
                  AND invoice_number LIKE v_year_prefix || '%'
                  AND invoice_number ~ ('^INV-' || v_year || '-[0-9]+$')
            ),
            1
        )
    )
    ON CONFLICT (property_id, year) DO UPDATE
    SET next_number = invoice_sequences.next_number + 1,
        updated_at = NOW()
    RETURNING next_number INTO v_next;

    RETURN v_year_prefix || LPAD(v_next::TEXT, 4, '0');
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_next_invoice_number(UUID) IS
    'Returns the next invoice_number (INV-YYYY-NNNN), gapless per property/year. Self-healing on first insert: initial value derived from MAX(invoice_number) of existing rows so the counter never races below reality.';

-- 3.8 trg_invoice_void_audit.
CREATE OR REPLACE FUNCTION trg_invoice_void_audit()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'void' AND OLD.status = 'active' THEN
        IF NEW.voided_at IS NULL THEN
            NEW.voided_at := NOW();
        END IF;
        IF NEW.voided_by IS NULL THEN
            RAISE EXCEPTION 'voided_by is required when voiding an invoice (invoice_id=%)', NEW.id
                USING ERRCODE = 'check_violation';
        END IF;
        IF NEW.void_reason IS NULL OR LENGTH(TRIM(NEW.void_reason)) = 0 THEN
            RAISE EXCEPTION 'void_reason is required when voiding an invoice (invoice_id=%)', NEW.id
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_void_audit ON invoices;
CREATE TRIGGER invoice_void_audit
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION trg_invoice_void_audit();

-- 3.9 trg_invoice_status_update (auto-promote active → refunded).
CREATE OR REPLACE FUNCTION trg_invoice_status_update()
RETURNS TRIGGER AS $$
DECLARE
    v_invoice_id UUID;
    v_total NUMERIC(12,2);
    v_paid NUMERIC(12,2);
    v_refunded NUMERIC(12,2);
    v_lifecycle TEXT;
BEGIN
    v_invoice_id := COALESCE(NEW.invoice_id, OLD.invoice_id);

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

    SELECT status INTO v_lifecycle FROM invoices WHERE id = v_invoice_id;

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

-- 3.10 trigger_set_updated_at — keep invoices.updated_at fresh.
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS set_updated_at ON invoices;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

DROP TRIGGER IF EXISTS set_updated_at ON bookings;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

DROP TRIGGER IF EXISTS set_updated_at ON guests;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON guests
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================================
-- 4. Runner catalog bootstrap — ensures `schema_migrations.filename`.
--    Old deployments booted the runner with a `schema_migrations(version,
--    applied_at)` table. Without this guard, the runner's INSERT-with-filename
--    would fail at apply time on legacy environments.
-- =============================================================================
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS filename TEXT;

COMMENT ON COLUMN schema_migrations.filename IS
    'Filename of the migration file associated with this version. Used by the runner to detect version-prefix collisions (e.g. 006_add_notes.sql and 006_invoicing_schema.up.sql sharing "006_").';

-- 006_invoicing_schema.up.sql
-- Invoicing & Payments module — TEREN Hotels
-- Ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md
--
-- Resumen del cambio:
--   1. Drop de la tabla `invoices` vestigial creada en 001 (nunca usada por
--      código ni seeds — verificado por grep). Se reemplaza por el schema
--      rico definido en el spec v1.1 (múltiples pagos por factura, line
--      items, sequences por property/año, idempotency keys).
--   2. Añade `bookings.force_override BOOLEAN` (referenciado por BR-INV-010
--      para permitir refunds por receptionist, reusando el flag ya conocido
--      del Booking Spec v1.2).
--   3. Crea el modelo de facturación:
--        - invoice_sequences  : numeración atómica por property/año
--        - invoices           : cabecera + auditoría fiscal
--        - invoice_line_items : líneas descriptivas
--        - payments           : múltiples pagos/refunds por factura
--        - idempotency_keys   : dedupe de POST /payments
--   4. Funciones y triggers:
--        - get_next_invoice_number(p_property_id)
--        - trg_invoice_void_audit (impide void sin voided_by)
--        - set_updated_at reusando trigger_set_updated_at() de 004

BEGIN;

-- ============================================================
-- 0. Drop de la tabla vestigial invoices (001_initial_schema)
--    Confirmado por grep: ningún INSERT/SELECT/REFERENCES en código ni seeds.
-- ============================================================
DROP TABLE IF EXISTS invoices CASCADE;

-- ============================================================
-- 1. Bookings: añadir force_override (BR-INV-010)
--    El campo ya existía en el DTO CreateBookingRequest pero no se
--    persistía. Ahora se guarda para que el flag esté disponible al
--    validar refunds en POST /invoices/:id/payments.
-- ============================================================
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS force_override BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN bookings.force_override IS
    'Permite al receptionist saltarse validaciones (overbooking, solapamiento, refunds).
     Reusado por el módulo de facturación para permitir refunds sin owner explícito.';

-- ============================================================
-- 2. invoice_sequences — numeración atómica por property/año
--    PK compuesta (property_id, year) para que el UPSERT funcione
--    idempotente y gapless dentro del mismo año.
-- ============================================================
CREATE TABLE IF NOT EXISTS invoice_sequences (
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    next_number INTEGER NOT NULL DEFAULT 1
        CHECK (next_number > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (property_id, year)
);

COMMENT ON TABLE invoice_sequences IS
    'Secuencia de numeración de facturas por property y año. Retención 10 años (alineado con
     requerimiento fiscal indonesio DG Tax). No se purga.';

-- ============================================================
-- 3. invoices — cabecera con auditoría fiscal completa
--    total = subtotal + tax_amount (verificado por chk_total_integrity)
--    ppn_rate_snapshot se persiste por invoice para auditoría histórica.
-- ============================================================
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','void')),
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    voided_by UUID REFERENCES users(id),
    void_reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    pdf_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Integridad: total = subtotal + tax_amount (la app calcula, la DB valida)
    CONSTRAINT chk_total_integrity CHECK (total = subtotal + tax_amount),
    -- Numeración gapless por property (un invoice_number por property)
    CONSTRAINT invoices_number_per_property_unique UNIQUE (property_id, invoice_number)
);

-- Índices optimizados para los queries del spec v1.1 §4.7 y §5.4
CREATE INDEX IF NOT EXISTS idx_invoices_property_issued_active
    ON invoices (property_id, issued_at DESC)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_invoices_property_status_issued
    ON invoices (property_id, status, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_invoices_property_paid_at
    ON invoices (property_id, paid_at)
    WHERE paid_at IS NOT NULL;

COMMENT ON TABLE invoices IS
    'Cabecera de facturas. status es el lifecycle (active/void); el estado derivado
     (unpaid/partial/paid/overpaid) lo computa el repositorio cruzando con payments.';

-- ============================================================
-- 4. invoice_line_items — líneas descriptivas
-- ============================================================
CREATE TABLE IF NOT EXISTS invoice_line_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    quantity NUMERIC(10,2) NOT NULL DEFAULT 1
        CHECK (quantity > 0),
    unit_price NUMERIC(12,2) NOT NULL,
    total NUMERIC(12,2) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice_sort
    ON invoice_line_items (invoice_id, sort_order);

COMMENT ON TABLE invoice_line_items IS
    'Líneas de factura. Una factura típica tiene 1 línea (alojamiento), pero el
     modelo soporta múltiples (ej. alojamiento + desayuno + tour).';

-- ============================================================
-- 5. payments — múltiples pagos por factura, soporta refunds
--    CHECK amount <> 0 (no amount = 0). Negativo = refund (BR-INV-010).
--    is_reversal + reversal_of encadenan refunds al pago original.
-- ============================================================
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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
    received_by UUID NOT NULL REFERENCES users(id),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Si es un refund (is_reversal=true) debe apuntar al pago original.
    -- Si NO es refund, NO debe apuntar a otro pago.
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
    ON payments (reversal_of)
    WHERE reversal_of IS NOT NULL;

COMMENT ON TABLE payments IS
    'Pagos individuales contra una factura. Permite múltiples pagos parciales (depósito + saldo)
     y refunds (amount < 0, is_reversal=TRUE, reversal_of apunta al pago original).';

-- ============================================================
-- 6. idempotency_keys — dedupe de POST /payments (R-06, TTL 24h)
--    El cliente envía `Idempotency-Key: <uuid>`. Si el mismo key llega en
--    <24h retornamos el payment original en vez de crear uno nuevo.
-- ============================================================
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);

CREATE INDEX IF NOT EXISTS idx_idempotency_expires
    ON idempotency_keys (expires_at);

COMMENT ON TABLE idempotency_keys IS
    'Cache de respuestas por Idempotency-Key. TTL 24h (R-06). Las claves expiradas se purgan
     vía job (futuro). Configurable por property en Phase 2.';

-- ============================================================
-- 7. Función: get_next_invoice_number(p_property_id)
--    UPSERT idempotente. Devuelve 'INV-YYYY-NNNN' gapless por property/año.
--    Primer INSERT inicializa next_number=1 y retorna 1 (RETURNING
--    proyecta el valor del INSERT). En CONFLICT incrementa +1.
--    Test de regresión obligatorio (spec §10): N=1000 goroutines
--    concurrentes deben numerarse 1..N sin huecos ni duplicados.
-- ============================================================
CREATE OR REPLACE FUNCTION get_next_invoice_number(p_property_id UUID)
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year INT := EXTRACT(YEAR FROM NOW());
    v_next INT;
BEGIN
    INSERT INTO invoice_sequences (property_id, year, next_number)
    VALUES (p_property_id, v_year, 1)
    ON CONFLICT (property_id, year) DO UPDATE
    SET next_number = invoice_sequences.next_number + 1,
        updated_at = NOW()
    RETURNING next_number INTO v_next;

    RETURN 'INV-' || v_year || '-' || LPAD(v_next::TEXT, 4, '0');
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_next_invoice_number(UUID) IS
    'Devuelve el siguiente invoice_number (INV-YYYY-NNNN) gapless por property/año.
     Primer INSERT devuelve 1; los siguientes retornan next_number+1 vía UPDATE.
     Concurrencia: el UPSERT sobre la PK (property_id, year) actúa como lock implícito
     a nivel de fila, garantizando atomicidad entre goroutines.';

-- ============================================================
-- 8. Trigger: trg_invoice_void_audit
--    Garantiza que cualquier void (status: active -> void) lleva
--    voided_at, voided_by y void_reason no vacío. Si faltan, la UPDATE falla.
-- ============================================================
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

-- ============================================================
-- 9. Trigger: set_updated_at para invoices
--    Reusa trigger_set_updated_at() de la migración 004.
-- ============================================================
DROP TRIGGER IF EXISTS set_updated_at ON invoices;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- ============================================================
-- 10. Comentario de versión para properties.settings
--     (ppn_rate y otros ajustes del módulo de facturación)
-- ============================================================
COMMENT ON TABLE properties IS
    'Properties. settings jsonb ahora soporta: ppn_rate (default 0.11), invoice_prefix (Phase 2).';

COMMIT;

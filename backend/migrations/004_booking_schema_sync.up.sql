-- 004_booking_schema_sync.up.sql
-- Sincronización del esquema para Booking Spec v1.2.0
-- Aplicar de forma incremental sobre 002_schema_corrections.up.sql
--
-- requires: guests.id, bookings.id, bookings.status, bookings.source
--
-- Idempotency contract:
--   * Guard schema at the top verifies the inputs from migrations 001/002
--     exist. Missing inputs are logged as warnings (not fatal).
--   * All ALTERs are wrapped with IF NOT EXISTS / DROP CONSTRAINT IF EXISTS
--     / CREATE OR REPLACE so re-applying the file leaves the schema unchanged.

-- =============================================================================
-- 0. Guard schema — verify the columns this migration mutates already exist.
-- =============================================================================
DO $$
DECLARE
    v_missing TEXT;
BEGIN
    SELECT string_agg(col, ', ' ORDER BY col)
      INTO v_missing
      FROM (
        VALUES
            ('guests.id'),
            ('bookings.id'),
            ('bookings.status'),
            ('bookings.source')
      ) AS required(col)
     WHERE NOT EXISTS (
            SELECT 1
              FROM information_schema.columns
             WHERE table_name = split_part(required.col, '.', 1)
               AND column_name = split_part(required.col, '.', 2)
     );

    IF v_missing IS NOT NULL THEN
        RAISE NOTICE '004 guard schema: missing prerequisites: %', v_missing;
    ELSE
        RAISE NOTICE '004 guard schema: all prerequisites present';
    END IF;
END$$;

-- 1. Guests: Modificaciones de columnas
ALTER TABLE guests ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE guests ADD COLUMN IF NOT EXISTS notes TEXT;

-- CHECK constraint for email format — guarded with IF EXISTS so re-apply is a no-op.
ALTER TABLE guests DROP CONSTRAINT IF EXISTS guests_email_format;
ALTER TABLE guests ADD CONSTRAINT guests_email_format
    CHECK (email IS NULL OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

-- 2. Bookings: añadir campos y aplicar restricciones correctas
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS adults INTEGER NOT NULL DEFAULT 1 CHECK (adults >= 1);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS children INTEGER NOT NULL DEFAULT 0 CHECK (children >= 0);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS original_amount NUMERIC(10,2) NOT NULL DEFAULT 0.00 CHECK (original_amount >= 0.00);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS original_currency CHAR(3) NOT NULL DEFAULT 'IDR' CHECK (original_currency ~ '^[A-Z]{3}$');
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(12,6) NOT NULL DEFAULT 1.000000 CHECK (exchange_rate > 0.00);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS payment_status TEXT NOT NULL DEFAULT 'pending';

-- CHECK constraints on existing/new columns.
-- Idempotent: drop before add so re-apply is safe.
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_payment_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_payment_status_check CHECK (payment_status IN ('pending','paid','partial'));

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_source_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_source_check CHECK (source IN ('walk_in','whatsapp','phone','booking_com','airbnb','agoda','traveloka','other'));

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (status IN ('confirmed','checked_in','checked_out','cancelled','no_show'));

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_total_amount_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_total_amount_check CHECK (total_amount >= 0.00);

-- room_id nullable — guarded so re-apply on an already-nullable column is a no-op.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_name = 'bookings'
           AND column_name = 'room_id'
           AND is_nullable = 'NO'
    ) THEN
        EXECUTE 'ALTER TABLE bookings ALTER COLUMN room_id DROP NOT NULL';
    END IF;
END$$;

-- 3. Inicialización de datos para compatibilidad de registros existentes.
--    COALESCE-style guard keeps the original values on re-apply.
UPDATE bookings
   SET original_amount   = total_amount,
       original_currency = 'IDR',
       exchange_rate     = 1.000000
 WHERE original_amount = 0.00
   AND total_amount <> 0.00;

-- 4. Creación de índices optimizados para el buscador y listados
DROP INDEX IF EXISTS idx_bookings_property_status_dates;
CREATE INDEX IF NOT EXISTS idx_bookings_property_status_dates
    ON bookings(property_id, status, check_in DESC, check_out)
    WHERE status NOT IN ('cancelled','no_show');

DROP INDEX IF EXISTS idx_guests_property_phone_email;
CREATE INDEX IF NOT EXISTS idx_guests_property_phone_email
    ON guests(property_id, phone, email);

-- 5. Configuración de Triggers para mantener updated_at.
--    CREATE OR REPLACE keeps re-apply idempotent even if the function exists.
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS set_updated_at ON bookings;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

DROP TRIGGER IF EXISTS set_updated_at ON guests;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON guests
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
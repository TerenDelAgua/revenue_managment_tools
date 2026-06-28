-- =============================================================================
-- Migration 009 — make get_next_invoice_number self-healing on insert
-- =============================================================================
--
-- Fix: in the INSERT branch of the UPSERT, compute the next number from the
-- actual MAX(invoice_number) of the existing rows for that property/year so
-- the counter always starts AT OR ABOVE reality. The ON CONFLICT branch is
-- untouched — once the row exists we trust the counter for gapless numbering
-- (preserves the §10 regression: 1000 concurrent goroutines must number 1..N).
-- =============================================================================

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
        -- Only runs when the row does NOT exist yet (no conflict).
        -- Derive the initial value from the highest invoice already on file
        -- for this property/year, so legacy / seeded rows don't clash.
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
    'Returns the next invoice_number (INV-YYYY-NNNN), gapless per property/year.
     Self-healing: on the first INSERT for a (property, year), the initial value
     is derived from MAX(invoice_number) of existing rows so the counter never
     races below reality (e.g. seeded INV-T-... rows). ON CONFLICT path is
     unchanged — concurrent inserts still serialise via the PK and yield
     gapless numbers (spec §10 regression: 1000 goroutines → 1..N).';
-- 013_guests_unique_phone_and_booking_exclusion.up.sql
-- Adds natural-key protections so the seed suite is idempotent and the
-- runtime cannot produce overlapping confirmed/checked_in bookings on
-- the same room.
--
-- Requires: schema_migrations.filename
--
-- Two protections:
--
-- 1. UNIQUE INDEX on guests(property_id, phone). The seed identifies
--    guests by phone number per property (no global uniqueness — different
--    properties can share a phone). The index is partial so guests with
--    a NULL phone do not collide with each other (acceptable: phones are
--    captured at the reception desk).
--
-- 2. EXCLUDE constraint on bookings that prevents two confirmed or
--    checked_in bookings from overlapping on the same room. Uses the
--    `btree_gist` extension so an `=` (UUID) and a range can coexist in
--    the same GIST index. Daterange semantics `[)` mirror the rest of
--    the codebase (check-out day is not occupied, so a 30/06-02/07 stay
--    does not collide with a 02/07-03/07 stay).
--
-- Cancelled and checked_out bookings are excluded from the constraint
-- so historical data can co-exist with new bookings.
--
-- Idempotent (per AGENTS.md rule 1): every ALTER / CREATE INDEX is
-- wrapped in IF [NOT] EXISTS or DROP IF EXISTS guards. Re-applying this
-- file is a no-op.

-- ============================================================
-- Guard schema (AGENTS.md rule 1): warn, never fatal, when
-- preconditions from earlier migrations are missing or different.
-- ============================================================
DO $$
DECLARE
    has_guests_table BOOLEAN;
    has_bookings_table BOOLEAN;
    has_phone_col BOOLEAN;
BEGIN
    SELECT EXISTS(
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'guests'
    ) INTO has_guests_table;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'bookings'
    ) INTO has_bookings_table;

    IF NOT has_guests_table THEN
        RAISE WARNING 'guard: table "guests" not found — 013 will no-op on UNIQUE INDEX creation';
    END IF;

    IF NOT has_bookings_table THEN
        RAISE WARNING 'guard: table "bookings" not found — 013 will no-op on EXCLUDE constraint creation';
    END IF;

    IF has_guests_table THEN
        SELECT EXISTS(
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema()
              AND table_name = 'guests'
              AND column_name = 'phone'
        ) INTO has_phone_col;

        IF NOT has_phone_col THEN
            RAISE WARNING 'guard: column "guests.phone" not found — 013 will no-op on UNIQUE INDEX creation';
        END IF;
    END IF;
END
$$;

-- ============================================================
-- 1. UNIQUE INDEX guests(property_id, phone) — partial on NOT NULL.
-- ============================================================
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'guests'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'guests'
          AND column_name = 'phone'
    ) THEN
        EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS guests_property_phone_unique
                 ON guests (property_id, phone)
                 WHERE phone IS NOT NULL';
    END IF;
END
$$;

-- ============================================================
-- 2. EXCLUDE constraint on bookings: no overlapping confirmed/checked_in
--    bookings per room.
-- ============================================================
DO $$
BEGIN
    -- btree_gist is required for combining a UUID `=` with a daterange
    -- `&&` inside one GIST index. Created unconditionally because it
    -- is idempotent and cheap.
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS btree_gist';

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'bookings'
    ) THEN
        EXECUTE 'ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_room_no_overlap';
        EXECUTE 'ALTER TABLE bookings
                 ADD CONSTRAINT bookings_room_no_overlap
                 EXCLUDE USING gist (
                     room_id WITH =,
                     daterange(check_in::date, check_out::date, ''[)'') WITH &&
                 )
                 WHERE (status IN (''confirmed'', ''checked_in''))';
    END IF;
END
$$;

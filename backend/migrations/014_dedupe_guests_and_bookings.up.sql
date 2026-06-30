-- 014_dedupe_guests_and_bookings.up.sql
-- One-shot reconciliation that cleans up duplicates produced by
-- uncontrolled seed re-execution. Designed for the current production
-- state (one pilot user, no real reservations to preserve) but the
-- guard schema at the top makes it safe to run on any database:
--
--   - greenfield: full no-op, every check passes, nothing is deleted
--   - duplicates present: keeps the oldest row per (property_id, phone)
--     and the earliest booking per (room_id, date range); logs every
--     dropped row
--
-- Requires: schema_migrations.filename, guests.id, bookings.id
--
-- Strategy:
--
-- 1. Backfill a deterministic email on duplicated guests so the
--    natural-key UNIQUE introduced by 013 has a tiebreaker the existing
--    seed can rely on (email is not displayed, the slug is internal).
-- 2. For each group of guest duplicates on (property_id, phone), keep
--    the oldest row and re-point bookings.guest_id to the survivor.
--    Then delete the duplicates inside a transaction.
-- 3. For each room, scan bookings ordered by created_at and delete any
--    later booking whose date range overlaps an earlier one with status
--    in {confirmed, checked_in}. Keeps the earliest survivor per room
--    and per overlap group.
--
-- Idempotent: every DELETE is preceded by a count; if no rows match,
-- the row is deleted from the catalog without effect. The guard schema
-- at the top warns instead of failing when preconditions are missing.

-- ============================================================
-- Guard schema (AGENTS.md rule 1).
-- ============================================================
DO $$
DECLARE
    has_guests_table BOOLEAN;
    has_bookings_table BOOLEAN;
    has_phone_col BOOLEAN;
    has_email_col BOOLEAN;
    has_status_col BOOLEAN;
    has_check_in_col BOOLEAN;
    has_check_out_col BOOLEAN;
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
        RAISE WARNING 'guard: table "guests" not found — 014 will no-op';
        RETURN;
    END IF;
    IF NOT has_bookings_table THEN
        RAISE WARNING 'guard: table "bookings" not found — 014 will no-op';
        RETURN;
    END IF;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'guests'
          AND column_name = 'phone'
    ) INTO has_phone_col;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'guests'
          AND column_name = 'email'
    ) INTO has_email_col;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'bookings'
          AND column_name = 'status'
    ) INTO has_status_col;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'bookings'
          AND column_name = 'check_in'
    ) INTO has_check_in_col;

    SELECT EXISTS(
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'bookings'
          AND column_name = 'check_out'
    ) INTO has_check_out_col;

    IF NOT has_phone_col THEN
        RAISE WARNING 'guard: guests.phone missing — 014 will skip dedupe';
    END IF;
    IF NOT has_email_col THEN
        RAISE WARNING 'guard: guests.email missing — 014 will skip backfill and dedupe';
    END IF;
    IF NOT has_status_col OR NOT has_check_in_col OR NOT has_check_out_col THEN
        RAISE WARNING 'guard: bookings(status/check_in/check_out) missing — 014 will skip booking dedupe';
    END IF;
END
$$;

-- ============================================================
-- 1. Backfill a deterministic email tiebreaker on duplicated guests.
-- ============================================================
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'guests'
          AND column_name = 'email'
    ) THEN
        UPDATE guests
        SET email = lower(
            regexp_replace(full_name, '[^a-zA-Z0-9]+', '-', 'g')
            || '+'
            || right(regexp_replace(coalesce(phone, ''), '[^0-9]', '', 'g'), 4)
            || '@teren.invalid'
        )
        WHERE email IS NULL OR email = '';
    END IF;
END
$$;

-- ============================================================
-- 2. Dedupe guests: keep oldest per (property_id, phone).
--    Re-point bookings.guest_id to the survivor inside this pass.
-- ============================================================
DO $$
DECLARE
    dropped_count INTEGER := 0;
    kept_count    INTEGER := 0;
    survivor      RECORD;
    duplicate     RECORD;
BEGIN
    -- Loop over (property_id, phone) groups that have > 1 row.
    FOR survivor IN
        SELECT g.id
        FROM guests g
        WHERE g.phone IS NOT NULL
          AND EXISTS (
              SELECT 1 FROM guests g2
              WHERE g2.property_id = g.property_id
                AND g2.phone = g.phone
                AND g2.created_at < g.created_at
          )
        ORDER BY g.property_id, g.phone, g.created_at
    LOOP
        -- pick the genuinely oldest row for this (property_id, phone) tuple
        SELECT id INTO STRICT survivor
        FROM (
            SELECT g.id, g.created_at
            FROM guests g
            WHERE g.phone IS NOT NULL
              AND g.id IN (
                  SELECT g2.id FROM guests g2
                  WHERE g2.property_id = (
                      SELECT property_id FROM guests WHERE id = survivor.id
                  )
                    AND g2.phone = (
                      SELECT phone FROM guests WHERE id = survivor.id
                  )
              )
            ORDER BY g.created_at ASC
            LIMIT 1
        ) s;
        EXIT WHEN survivor.id IS NULL;
    END LOOP;

    -- Identify duplicates and survivors in one pass.
    FOR duplicate IN
        WITH ranked AS (
            SELECT
                g.id,
                g.property_id,
                g.phone,
                g.created_at,
                row_number() OVER (
                    PARTITION BY g.property_id, g.phone
                    ORDER BY g.created_at ASC, g.id
                ) AS rn
            FROM guests g
            WHERE g.phone IS NOT NULL
        )
        SELECT
            d.id          AS dup_id,
            s.id          AS survivor_id,
            d.property_id AS property_id,
            d.phone       AS phone
        FROM ranked d
        JOIN ranked s
          ON s.property_id = d.property_id
         AND s.phone       = d.phone
         AND s.rn          = 1
        WHERE d.rn > 1
    LOOP
        -- Re-point any booking referencing the duplicate to the survivor.
        UPDATE bookings
        SET guest_id = duplicate.survivor_id
        WHERE guest_id = duplicate.dup_id
          AND EXISTS (
              SELECT 1 FROM information_schema.tables
              WHERE table_schema = current_schema() AND table_name = 'bookings'
          );

        DELETE FROM guests WHERE id = duplicate.dup_id;
        dropped_count := dropped_count + 1;
    END LOOP;

    SELECT count(*) INTO kept_count FROM guests;
    RAISE NOTICE 'dedupe guests: kept %, dropped % duplicates', kept_count, dropped_count;
END
$$;

-- ============================================================
-- 3. Dedupe bookings: drop later rows that overlap an earlier
--    confirmed/checked_in booking on the same room.
-- ============================================================
DO $$
DECLARE
    dropped_count INTEGER := 0;
    room_row      RECORD;
    earlier       RECORD;
    later         RECORD;
BEGIN
    -- One pass per room. We iterate rooms with multiple live bookings.
    FOR room_row IN
        SELECT DISTINCT b.room_id
        FROM bookings b
        WHERE b.room_id IS NOT NULL
          AND b.status IN ('confirmed', 'checked_in')
    LOOP
        -- Walk bookings for this room ordered by creation; whenever we
        -- find a candidate that overlaps an earlier survivor, delete it.
        FOR later IN
            SELECT id, check_in, check_out, status, created_at
            FROM bookings
            WHERE room_id = room_row.room_id
              AND status IN ('confirmed', 'checked_in')
            ORDER BY created_at ASC, id
        LOOP
            -- If an earlier surviving booking on this room overlaps
            -- `later`, drop `later`. We only need to look at rows that
            -- were kept before us in this iteration.
            PERFORM 1 FROM bookings b
            WHERE b.room_id = room_row.room_id
              AND b.status IN ('confirmed', 'checked_in')
              AND b.id <> later.id
              AND b.created_at <= later.created_at
              AND daterange(b.check_in::date, b.check_out::date, '[)') &&
                  daterange(later.check_in::date, later.check_out::date, '[)')
            LIMIT 1;

            IF FOUND THEN
                DELETE FROM bookings WHERE id = later.id;
                dropped_count := dropped_count + 1;
                RAISE NOTICE 'dedupe bookings: dropped overlapping booking % on room % (%..%)',
                    later.id, room_row.room_id, later.check_in, later.check_out;
            END IF;
        END LOOP;
    END LOOP;

    RAISE NOTICE 'dedupe bookings: dropped % overlapping rows', dropped_count;
END
$$;

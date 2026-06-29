-- 002_schema_corrections.up.sql
-- Corrige discrepancias entre schema inicial y Spec FMB-001
-- Ref: BR-01, BR-08, AC-04, AC-09, OQ-01 (deferred)
--
-- requires: rooms.floor_id, rooms.number, floors.property_id, bookings.room_id
--
-- Idempotency contract (enforced by the runner + this file):
--   * Guard schema at the top verifies the inputs from migration 001 exist.
--     Missing inputs are reported as warnings (DO block + RAISE NOTICE) and
--     the file proceeds so a partial / legacy catalog can self-heal.
--   * Every ALTER is wrapped with IF EXISTS / IF NOT EXISTS or a DO block
--     that introspects pg_catalog before mutating.
--   * Re-applying this file leaves the database in the same shape.

-- =============================================================================
-- 0. Guard schema — verify migration 001 inputs exist.
--    Each check is non-fatal: if the column is missing we log a warning and
--    let the section below either create it (where possible) or skip it.
-- =============================================================================
DO $$
DECLARE
    v_missing TEXT;
BEGIN
    SELECT string_agg(col, ', ' ORDER BY col)
      INTO v_missing
      FROM (
        VALUES
            ('rooms.floor_id'),
            ('rooms.number'),
            ('floors.property_id'),
            ('bookings.room_id')
      ) AS required(col)
     WHERE NOT EXISTS (
            SELECT 1
              FROM information_schema.columns
             WHERE table_name = split_part(required.col, '.', 1)
               AND column_name = split_part(required.col, '.', 2)
     );

    IF v_missing IS NOT NULL THEN
        RAISE NOTICE '002 guard schema: missing prerequisites from migration 001: %', v_missing;
    ELSE
        RAISE NOTICE '002 guard schema: all prerequisites present';
    END IF;
END$$;

-- ============================================================
-- 1. Desnormalizar property_id en rooms (para BR-08 + RLS futuro)
-- ============================================================
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS property_id UUID;

UPDATE rooms
SET property_id = f.property_id
FROM floors f
WHERE rooms.floor_id = f.id
  AND rooms.property_id IS NULL;

-- Enforce NOT NULL only when the column has no remaining NULLs, so re-apply
-- on a fully-shaped table is a no-op (idempotent SET NOT NULL guard).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_name = 'rooms'
           AND column_name = 'property_id'
           AND is_nullable = 'YES'
    ) AND NOT EXISTS (
        SELECT 1 FROM rooms WHERE property_id IS NULL
    ) THEN
        EXECUTE 'ALTER TABLE rooms ALTER COLUMN property_id SET NOT NULL';
    END IF;
END$$;

-- Idempotent: drop both legacy and current constraint names before re-adding.
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_property_fk;
ALTER TABLE rooms
    ADD CONSTRAINT rooms_property_fk
    FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;

-- ============================================================
-- 2. BR-08: Room number único por PROPERTY (no por floor)
--    Idempotent: drop both legacy and current constraint names first.
-- ============================================================
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_floor_id_number_key;
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_property_number_unique;
ALTER TABLE rooms
    ADD CONSTRAINT rooms_property_number_unique
    UNIQUE (property_id, number);

-- =============================================================================
-- 3. BR-01 / AC-04: Una sola habitación por celda del grid
--    Idempotent: drop before add because ADD CONSTRAINT with the same name
--    would otherwise fail on re-apply.
-- =============================================================================
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_floor_position_unique;
ALTER TABLE rooms
    ADD CONSTRAINT rooms_floor_position_unique
    UNIQUE (floor_id, pos_x, pos_y);

-- =============================================================================
-- 4. room_blocks: ENUM tipado + CHECK de fechas
--    The CREATE TYPE and ALTER TYPE are idempotent guards because a second
--    application (e.g. legacy catalog without `filename`) would otherwise
--    fail with "type already exists" / "type cannot be altered because it
--    is still in use".
-- =============================================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'block_reason') THEN
        CREATE TYPE block_reason AS ENUM ('maintenance', 'owner_use', 'out_of_service');
    END IF;
END$$;

-- Re-issue the ALTER TYPE so that subsequent migrations stay no-ops.
-- Guarded: skip when room_blocks table is missing (legacy catalog state).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables
               WHERE table_schema = 'public' AND table_name = 'room_blocks') THEN
        EXECUTE $SQL$
            ALTER TABLE room_blocks
                ALTER COLUMN reason TYPE block_reason
                USING reason::block_reason
        $SQL$;
    ELSE
        RAISE NOTICE '002 section 4: room_blocks table missing, skipping ALTER TYPE';
    END IF;
END$$;

ALTER TABLE room_blocks DROP CONSTRAINT IF EXISTS room_blocks_dates_check;
ALTER TABLE room_blocks
    ADD CONSTRAINT room_blocks_dates_check
    CHECK (end_date > start_date);

-- ============================================================
-- 5. Índices de rendimiento para GET /map (AC-09: < 1.5s en 4G)
-- ============================================================
-- Cobertura del query de disponibilidad (overlap de rangos)
CREATE INDEX IF NOT EXISTS idx_rooms_property_floor_position
    ON rooms (property_id, floor_id, pos_y, pos_x);

CREATE INDEX IF NOT EXISTS idx_room_blocks_room_dates
    ON room_blocks (room_id, start_date, end_date);

CREATE INDEX IF NOT EXISTS idx_bookings_room_dates_active
    ON bookings (room_id, check_in, check_out)
    WHERE status NOT IN ('cancelled', 'no_show');

-- Para listado rápido de floors por property (tabs del mapa)
CREATE INDEX IF NOT EXISTS idx_floors_property_sort
    ON floors (property_id, sort_order, floor_number);

-- Permite reservas sin habitación asignada (Spec FMB-001 / PMS flow).
-- Guarded: only flip NOT NULL → NULL when the column is currently NOT NULL,
-- otherwise re-apply is a no-op (idempotent).
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
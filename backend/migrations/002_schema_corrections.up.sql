-- 003_schema_corrections.up.sql
-- Corrige discrepancias entre schema inicial y Spec FMB-001
-- Ref: BR-01, BR-08, AC-04, AC-09, OQ-01 (deferred)

-- ============================================================
-- 1. Desnormalizar property_id en rooms (para BR-08 + RLS futuro)
-- ============================================================
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS property_id UUID;

UPDATE rooms
SET property_id = f.property_id
FROM floors f
WHERE rooms.floor_id = f.id
  AND rooms.property_id IS NULL;

ALTER TABLE rooms ALTER COLUMN property_id SET NOT NULL;
ALTER TABLE rooms
    ADD CONSTRAINT rooms_property_fk
    FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;

-- ============================================================
-- 2. BR-08: Room number único por PROPERTY (no por floor)
-- ============================================================
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_floor_id_number_key;
ALTER TABLE rooms
    ADD CONSTRAINT rooms_property_number_unique
    UNIQUE (property_id, number);

-- ============================================================
-- 3. BR-01 / AC-04: Una sola habitación por celda del grid
-- ============================================================
ALTER TABLE rooms
    ADD CONSTRAINT rooms_floor_position_unique
    UNIQUE (floor_id, pos_x, pos_y);

-- ============================================================
-- 4. room_blocks: ENUM tipado + CHECK de fechas
-- ============================================================
CREATE TYPE block_reason AS ENUM ('maintenance', 'owner_use', 'out_of_service');

ALTER TABLE room_blocks
    ALTER COLUMN reason TYPE block_reason
    USING reason::block_reason;

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
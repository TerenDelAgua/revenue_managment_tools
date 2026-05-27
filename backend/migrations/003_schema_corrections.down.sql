-- 003_schema_corrections.down.sql
-- Rollback de 003_schema_corrections.up.sql

DROP INDEX IF EXISTS idx_floors_property_sort;
DROP INDEX IF EXISTS idx_bookings_room_dates_active;
DROP INDEX IF EXISTS idx_room_blocks_room_dates;
DROP INDEX IF EXISTS idx_rooms_property_floor_position;

ALTER TABLE room_blocks DROP CONSTRAINT IF EXISTS room_blocks_dates_check;
ALTER TABLE room_blocks ALTER COLUMN reason TYPE TEXT;
DROP TYPE IF EXISTS block_reason;

ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_floor_position_unique;
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_property_number_unique;
ALTER TABLE rooms ADD CONSTRAINT rooms_floor_id_number_key UNIQUE (floor_id, number);

ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_property_fk;
ALTER TABLE rooms DROP COLUMN IF EXISTS property_id;

-- Revertir room_id nullable
ALTER TABLE bookings ALTER COLUMN room_id SET NOT NULL;
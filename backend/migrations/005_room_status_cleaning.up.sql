-- 005_room_status_cleaning.up.sql
-- Añade el estado "cleaning" al dominio persistente de rooms.
--
-- Contexto:
--   - rooms.status es TEXT y solo guarda el estado "persistente" del room.
--     Los estados derivados (occupied/blocked/pending) los computa el repositorio
--     en GetMapWithAvailability cruzando con bookings/room_blocks.
--   - "cleaning" es un estado operacional: housekeeping en curso tras check-out
--     o antes del próximo check-in. Mientras la habitación está en cleaning NO
--     debe ser vendible. Es efímero (se quita al acabar la limpieza).
--
-- Diseño:
--   - Se añade un CHECK constraint con el dominio canónico de status.
--   - Pre-chequeo defensivo: cualquier valor fuera del dominio se normaliza a
--     'active' (estado por defecto) para que el ALTER no rompa filas legacy.
--   - El repositorio y servicio se actualizan en migraciones/swaps de código
--     posteriores (ver 005_cleaning_* en backend/internal/...).
--
-- Refs: BR-08 (constraint per property), FMB-001 §3.1 (status derived vs persistent).

BEGIN;

-- 1. Normalizar valores fuera del dominio canónico antes de añadir la restricción.
UPDATE rooms
SET status = 'active'
WHERE status IS NULL
   OR status NOT IN ('active', 'inactive', 'maintenance', 'cleaning');

-- 2. CHECK constraint no destructivo (idempotente con IF NOT EXISTS via DO block).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'rooms_status_check'
    ) THEN
        ALTER TABLE rooms
            ADD CONSTRAINT rooms_status_check
            CHECK (status IN ('active', 'inactive', 'maintenance', 'cleaning'));
    END IF;
END
$$;

-- 3. Comentario de columna para reflejar el dominio canónico.
COMMENT ON COLUMN rooms.status IS
    'Estado persistente del room. Dominio: active | inactive | maintenance | cleaning.
     Estados como occupied/pending/blocked se derivan en GetMapWithAvailability
     cruzando con bookings y room_blocks.';

COMMIT;

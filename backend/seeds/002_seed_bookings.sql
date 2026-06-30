-- 002_seed_bookings.sql
-- Seed de reservas para probar flujos de asignación y check-in/out.
--
-- Idempotente: re-ejecutable sin duplicar huéspedes ni reservas.
--
-- Estrategias (per AGENTS.md rule 1):
--   - Guests: INSERT … ON CONFLICT (property_id, phone) DO NOTHING.
--     El índice natural-key lo crea la migración 013, por lo que este
--     seed depende de que 013 se haya aplicado. Si no existe, el
--     INSERT falla con SQLSTATE 23505 y la transacción hace rollback
--     completo (incluidos los bookings previos en este mismo archivo) —
--     preferible a fallar silenciosamente a mitad de archivo.
--   - Bookings: pre-check via NOT EXISTS sobre (room_id, daterange)
--     solapado. Si ya hay una reserva confirmada o checked_in para la
--     misma habitación en el mismo rango, salta con RAISE NOTICE en
--     lugar de fallar. La EXCLUDE constraint introducida por la
--     migración 013 actúa como red de seguridad a nivel DB.

DO $$
DECLARE
    v_prop_id   uuid;
    v_r101_id   uuid;
    v_r102_id   uuid;
    v_r103_id   uuid;
    v_guest1_id uuid;
    v_guest2_id uuid;
    v_guest3_id uuid;
    v_user_id   uuid;
BEGIN
    SELECT id INTO v_prop_id FROM properties WHERE slug = 'teren-test-hotel';
    SELECT id INTO v_r101_id FROM rooms   WHERE number = '101';
    SELECT id INTO v_r102_id FROM rooms   WHERE number = '102';
    SELECT id INTO v_r103_id FROM rooms   WHERE number = '103';

    -- Users: ya era idempotente con ON CONFLICT DO NOTHING.
    INSERT INTO users (property_id, name, email, role)
    VALUES (v_prop_id, 'Admin User', 'admin@teren.dev', 'owner')
    ON CONFLICT DO NOTHING
    RETURNING id INTO v_user_id;

    IF v_user_id IS NULL THEN
        SELECT id INTO v_user_id FROM users LIMIT 1;
    END IF;

    -- Guests: idempotente via 013's UNIQUE(property_id, phone).
    INSERT INTO guests (property_id, full_name, phone, nationality)
    VALUES (v_prop_id, 'Juan Pérez', '+62812345678', 'ESP')
    ON CONFLICT (property_id, phone) DO NOTHING
    RETURNING id INTO v_guest1_id;

    IF v_guest1_id IS NULL THEN
        SELECT id INTO v_guest1_id
        FROM guests
        WHERE property_id = v_prop_id AND phone = '+62812345678';
    END IF;

    INSERT INTO guests (property_id, full_name, phone, nationality)
    VALUES (v_prop_id, 'Maria Garcia', '+62812345679', 'MEX')
    ON CONFLICT (property_id, phone) DO NOTHING
    RETURNING id INTO v_guest2_id;

    IF v_guest2_id IS NULL THEN
        SELECT id INTO v_guest2_id
        FROM guests
        WHERE property_id = v_prop_id AND phone = '+62812345679';
    END IF;

    INSERT INTO guests (property_id, full_name, phone, nationality)
    VALUES (v_prop_id, 'John Doe', '+62812345680', 'USA')
    ON CONFLICT (property_id, phone) DO NOTHING
    RETURNING id INTO v_guest3_id;

    IF v_guest3_id IS NULL THEN
        SELECT id INTO v_guest3_id
        FROM guests
        WHERE property_id = v_prop_id AND phone = '+62812345680';
    END IF;

    -- Bookings: pre-check overlap. Cada bloque hace su propio
    -- INSERT-IF-NOT-OVERLAPPING, así si uno se salta los otros
    -- siguen aplicándose.

    -- 1. Reserva CHECKED_IN (room 101).
    IF NOT EXISTS (
        SELECT 1 FROM bookings
        WHERE room_id = v_r101_id
          AND status IN ('confirmed', 'checked_in')
          AND daterange(check_in::date, check_out::date, '[)') &&
              daterange(CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE + INTERVAL '2 days', '[)')
    ) THEN
        INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
        VALUES (v_prop_id, v_r101_id, v_guest1_id, v_user_id,
                CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE + INTERVAL '2 days',
                500000, 'walk_in', 'checked_in');
    ELSE
        RAISE NOTICE 'seed: skipping booking for room 101 — overlapping confirmed/checked_in range already exists';
    END IF;

    -- 2. Reserva CONFIRMED (room 102).
    IF NOT EXISTS (
        SELECT 1 FROM bookings
        WHERE room_id = v_r102_id
          AND status IN ('confirmed', 'checked_in')
          AND daterange(check_in::date, check_out::date, '[)') &&
              daterange(CURRENT_DATE + INTERVAL '1 day', CURRENT_DATE + INTERVAL '3 days', '[)')
    ) THEN
        INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
        VALUES (v_prop_id, v_r102_id, v_guest2_id, v_user_id,
                CURRENT_DATE + INTERVAL '1 day', CURRENT_DATE + INTERVAL '3 days',
                600000, 'whatsapp', 'confirmed');
    ELSE
        RAISE NOTICE 'seed: skipping booking for room 102 — overlapping confirmed/checked_in range already exists';
    END IF;

    -- 3. Reserva CONFIRMED (room 103).
    IF NOT EXISTS (
        SELECT 1 FROM bookings
        WHERE room_id = v_r103_id
          AND status IN ('confirmed', 'checked_in')
          AND daterange(check_in::date, check_out::date, '[)') &&
              daterange(CURRENT_DATE, CURRENT_DATE + INTERVAL '2 days', '[)')
    ) THEN
        INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
        VALUES (v_prop_id, v_r103_id, v_guest3_id, v_user_id,
                CURRENT_DATE, CURRENT_DATE + INTERVAL '2 days',
                750000, 'booking_com', 'confirmed');
    ELSE
        RAISE NOTICE 'seed: skipping booking for room 103 — overlapping confirmed/checked_in range already exists';
    END IF;
END
$$;

-- 004_seed_bookings.sql
-- Seed de reservas para probar flujos de asignación y check-in/out

DO $$
DECLARE
    v_prop_id uuid;
    v_r101_id uuid;
    v_r102_id uuid;
    v_r103_id uuid;
    v_guest1_id uuid;
    v_guest2_id uuid;
    v_guest3_id uuid;
    v_user_id uuid;
BEGIN
    SELECT id INTO v_prop_id FROM properties WHERE slug = 'teren-test-hotel';
    SELECT id INTO v_r101_id FROM rooms WHERE number = '101';
    SELECT id INTO v_r102_id FROM rooms WHERE number = '102';
    SELECT id INTO v_r103_id FROM rooms WHERE number = '103';
    
    INSERT INTO users (property_id, name, email, role) 
    VALUES (v_prop_id, 'Admin User', 'admin@teren.dev', 'admin')
    ON CONFLICT DO NOTHING
    RETURNING id INTO v_user_id;

    IF v_user_id IS NULL THEN
        SELECT id INTO v_user_id FROM users LIMIT 1;
    END IF;

    INSERT INTO guests (property_id, full_name, phone, nationality) VALUES (v_prop_id, 'Juan Pérez', '+62812345678', 'ESP') RETURNING id INTO v_guest1_id;
    INSERT INTO guests (property_id, full_name, phone, nationality) VALUES (v_prop_id, 'Maria Garcia', '+62812345679', 'MEX') RETURNING id INTO v_guest2_id;
    INSERT INTO guests (property_id, full_name, phone, nationality) VALUES (v_prop_id, 'John Doe', '+62812345680', 'USA') RETURNING id INTO v_guest3_id;

    -- 1. Reserva CHECKED_IN (Ocupada - Rojo)
    INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
    VALUES (v_prop_id, v_r101_id, v_guest1_id, v_user_id, CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE + INTERVAL '2 days', 500000, 'walk_in', 'checked_in');

    -- 2. Reserva CONFIRMED (Pendiente - Ámbar)
    INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
    VALUES (v_prop_id, v_r102_id, v_guest2_id, v_user_id, CURRENT_DATE + INTERVAL '1 day', CURRENT_DATE + INTERVAL '3 days', 600000, 'whatsapp', 'confirmed');

    -- 3. Otra reserva CONFIRMED (Pendiente - Ámbar)
    INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
    VALUES (v_prop_id, v_r103_id, v_guest3_id, v_user_id, CURRENT_DATE, CURRENT_DATE + INTERVAL '2 days', 750000, 'booking_com', 'confirmed');
END $$;
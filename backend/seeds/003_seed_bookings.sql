-- 004_seed_bookings.sql
-- Seed de reservas para probar estados: Occupied (Red), Pending (Amber), Available (Green)

-- Insert a user (owner) if not exists
INSERT INTO users (property_id, name, email, role)
SELECT (SELECT id FROM properties WHERE slug='teren-test-hotel'), 'Admin', 'admin@teren.com', 'owner'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE role='owner' AND email='admin@teren.com');

-- 1. Reserva Ocupada (Checked In) - Hoy hasta mañana
WITH 
    prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel'),
    guest1 AS (
        INSERT INTO guests (property_id, full_name, phone, nationality)
        SELECT id, 'Juan Pérez', '+62812345678', 'ESP' FROM prop RETURNING id
    ),
    user_owner AS (SELECT id FROM users WHERE role = 'owner' LIMIT 1),
    room101 AS (SELECT id FROM rooms WHERE number = '101')
INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
SELECT p.id, r101.id, g1.id, u.id,
       CURRENT_DATE, CURRENT_DATE + INTERVAL '1 day',
       500000, 'walk_in', 'checked_in'
FROM prop p, guest1 g1, user_owner u, room101 r101;

-- 2. Reserva Pendiente (Confirmed) - Mañana hasta pasado mañana
WITH 
    prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel'),
    guest2 AS (
        INSERT INTO guests (property_id, full_name, phone, nationality)
        SELECT id, 'Maria Garcia', '+62812345679', 'MEX' FROM prop RETURNING id
    ),
    user_owner AS (SELECT id FROM users WHERE role = 'owner' LIMIT 1),
    room102 AS (SELECT id FROM rooms WHERE number = '102')
INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
SELECT p.id, r102.id, g2.id, u.id,
       CURRENT_DATE + INTERVAL '1 day', CURRENT_DATE + INTERVAL '3 days',
       600000, 'whatsapp', 'confirmed'
FROM prop p, guest2 g2, user_owner u, room102 r102;

-- 3. Reserva Futura (Confirmed) - La semana que viene
WITH 
    prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel'),
    guest3 AS (
        INSERT INTO guests (property_id, full_name, phone, nationality)
        SELECT id, 'John Doe', '+62812345680', 'USA' FROM prop RETURNING id
    ),
    user_owner AS (SELECT id FROM users WHERE role = 'owner' LIMIT 1),
    room103 AS (SELECT id FROM rooms WHERE number = '103')
INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
SELECT p.id, r103.id, g3.id, u.id,
       CURRENT_DATE + INTERVAL '7 days', CURRENT_DATE + INTERVAL '10 days',
       750000, 'booking_com', 'confirmed'
FROM prop p, guest3 g3, user_owner u, room103 r103;
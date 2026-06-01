-- 002_seed_data.sql
-- Seed para TEREN Test Hotel
-- Grid: 12 cols × 20 rows (Spec FMB-001 §4.1)

-- 1. Property
INSERT INTO properties (id, name, slug, currency, timezone, settings)
VALUES (
    '89ce1655-d0c6-417a-8c69-3ad59241e0d0',
    'TEREN Test Hotel',
    'teren-test-hotel',
    'IDR',
    'Asia/Jakarta',
    '{"maxGuestsPerRoom": 4, "defaultCheckInTime": "14:00", "defaultCheckOutTime": "12:00"}'::jsonb
) ON CONFLICT (slug) DO NOTHING;

-- 2. Floors
WITH prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel')
INSERT INTO floors (property_id, floor_number, label, sort_order)
SELECT id, 1, 'Ground Floor', 1 FROM prop
UNION ALL SELECT id, 2, 'First Floor', 2 FROM prop
UNION ALL SELECT id, 3, 'Second Floor', 3 FROM prop
ON CONFLICT DO NOTHING;

-- 3. Room type
INSERT INTO room_types (property_id, name, max_occupancy)
SELECT id, 'Standard Room', 2
FROM properties WHERE slug = 'teren-test-hotel'
ON CONFLICT DO NOTHING;

-- 4. Rooms — coordenadas de GRID (no píxeles)
-- Layout: filas horizontales empezando en (x=0, y=0)
WITH
    prop   AS (SELECT id AS property_id FROM properties WHERE slug = 'teren-test-hotel'),
    floor1 AS (SELECT id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 1),
    floor2 AS (SELECT id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 2),
    floor3 AS (SELECT id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 3),
    rt     AS (SELECT id FROM room_types r, prop p WHERE r.property_id = p.property_id LIMIT 1)
INSERT INTO rooms (property_id, floor_id, room_type_id, number, status, pos_x, pos_y)
-- Ground Floor · y=0 · 5 rooms
SELECT p.property_id, f1.id, rt.id, '101', 'active', 0, 0 FROM prop p, floor1 f1, rt
UNION ALL SELECT p.property_id, f1.id, rt.id, '102', 'active', 1, 0 FROM prop p, floor1 f1, rt
UNION ALL SELECT p.property_id, f1.id, rt.id, '103', 'active', 2, 0 FROM prop p, floor1 f1, rt
UNION ALL SELECT p.property_id, f1.id, rt.id, '104', 'active', 3, 0 FROM prop p, floor1 f1, rt
UNION ALL SELECT p.property_id, f1.id, rt.id, '105', 'active', 4, 0 FROM prop p, floor1 f1, rt
-- First Floor · y=0 · 4 rooms
UNION ALL SELECT p.property_id, f2.id, rt.id, '201', 'active', 0, 0 FROM prop p, floor2 f2, rt
UNION ALL SELECT p.property_id, f2.id, rt.id, '202', 'active', 1, 0 FROM prop p, floor2 f2, rt
UNION ALL SELECT p.property_id, f2.id, rt.id, '203', 'active', 2, 0 FROM prop p, floor2 f2, rt
UNION ALL SELECT p.property_id, f2.id, rt.id, '204', 'active', 3, 0 FROM prop p, floor2 f2, rt
-- Second Floor · y=0 · 3 rooms
UNION ALL SELECT p.property_id, f3.id, rt.id, '301', 'active', 0, 0 FROM prop p, floor3 f3, rt
UNION ALL SELECT p.property_id, f3.id, rt.id, '302', 'active', 1, 0 FROM prop p, floor3 f3, rt
UNION ALL SELECT p.property_id, f3.id, rt.id, '303', 'active', 2, 0 FROM prop p, floor3 f3, rt
ON CONFLICT DO NOTHING;
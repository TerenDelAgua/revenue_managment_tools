-- Seed data for TEREN Hotels
-- Run this migration after 001_initial_schema.sql

-- Create a test property
INSERT INTO properties (id, name, slug, currency, timezone, settings)
VALUES (
    '89ce1655-d0c6-417a-8c69-3ad59241e0d0',
    'TEREN Test Hotel',
    'teren-test-hotel',
    'IDR',
    'Asia/Jakarta',
    '{"maxGuestsPerRoom": 4, "defaultCheckInTime": "14:00", "defaultCheckOutTime": "12:00"}'::jsonb
);

-- Get the property ID for reference
WITH prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel')

-- Create test floors
INSERT INTO floors (property_id, floor_number, label, sort_order)
SELECT id, 1, 'Ground Floor', 1 FROM prop
UNION ALL
SELECT id, 2, 'First Floor', 2 FROM prop
UNION ALL
SELECT id, 3, 'Second Floor', 3 FROM prop;

-- Create a test room type (we need this for rooms)
INSERT INTO room_types (property_id, name, max_occupancy)
SELECT id, 'Standard Room', 2 FROM properties WHERE slug = 'teren-test-hotel';

-- Create test rooms
WITH 
prop AS (SELECT id AS property_id FROM properties WHERE slug = 'teren-test-hotel'),
floor1 AS (SELECT id AS floor_id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 1),
floor2 AS (SELECT id AS floor_id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 2),
floor3 AS (SELECT id AS floor_id FROM floors f, prop p WHERE f.property_id = p.property_id AND f.floor_number = 3),
rt AS (SELECT id AS room_type_id FROM room_types rt, prop p WHERE rt.property_id = p.property_id)

INSERT INTO rooms (floor_id, room_type_id, number, status, pos_x, pos_y)
-- Ground Floor rooms
SELECT floor1.floor_id, rt.room_type_id, '101', 'available', 50, 50 FROM floor1, rt
UNION ALL
SELECT floor1.floor_id, rt.room_type_id, '102', 'available', 200, 50 FROM floor1, rt
UNION ALL
SELECT floor1.floor_id, rt.room_type_id, '103', 'maintenance', 350, 50 FROM floor1, rt
UNION ALL
SELECT floor1.floor_id, rt.room_type_id, '104', 'available', 500, 50 FROM floor1, rt
UNION ALL
SELECT floor1.floor_id, rt.room_type_id, '105', 'occupied', 650, 50 FROM floor1, rt
-- First Floor rooms
UNION ALL
SELECT floor2.floor_id, rt.room_type_id, '201', 'available', 50, 50 FROM floor2, rt
UNION ALL
SELECT floor2.floor_id, rt.room_type_id, '202', 'available', 200, 50 FROM floor2, rt
UNION ALL
SELECT floor2.floor_id, rt.room_type_id, '203', 'available', 350, 50 FROM floor2, rt
UNION ALL
SELECT floor2.floor_id, rt.room_type_id, '204', 'available', 500, 50 FROM floor2, rt
-- Second Floor rooms
UNION ALL
SELECT floor3.floor_id, rt.room_type_id, '301', 'available', 50, 50 FROM floor3, rt
UNION ALL
SELECT floor3.floor_id, rt.room_type_id, '302', 'available', 200, 50 FROM floor3, rt
UNION ALL
SELECT floor3.floor_id, rt.room_type_id, '303', 'available', 350, 50 FROM floor3, rt;

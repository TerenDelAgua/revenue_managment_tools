-- 005_seed_room_types.sql
-- Seed de tipos de habitación para TEREN Test Hotel
-- Cubre categorías reales en hospitality indonesia (losmen, boutique, guesthouses)
-- Idempotente: seguro ejecutar múltiples veces

WITH prop AS (SELECT id FROM properties WHERE slug = 'teren-test-hotel')
INSERT INTO room_types (property_id, name, max_occupancy)
SELECT prop.id, vals.name, vals.max_occupancy
FROM prop
CROSS JOIN (
    VALUES 
        ('Standard Room', 2),
        ('Deluxe Room', 2),
        ('Twin Room', 2),
        ('Family Room', 4),
        ('Economy Room', 2),
        ('Suite', 3),
        ('Dormitory (6 Beds)', 6),
        ('Private Bungalow', 3)
) AS vals(name, max_occupancy)
WHERE NOT EXISTS (
    SELECT 1 FROM room_types rt 
    WHERE rt.property_id = prop.id AND rt.name = vals.name
);
-- 004_seed_invoicing.sql
-- Seed del módulo de facturación.
-- Crea 2 facturas de ejemplo (pagada + parcial) vinculadas a las reservas
-- existentes del seed anterior, con sus line_items y payments.
--
-- Ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md
--
-- Este seed es IDPOTENTE: usa INSERT ... ON CONFLICT (invoice_number) DO NOTHING
-- para que se pueda ejecutar varias veces sin duplicar filas.

DO $$
DECLARE
    v_prop_id uuid;
    v_user_id uuid;
    v_booking_101_id uuid;
    v_booking_102_id uuid;
    v_invoice_paid_id uuid;
    v_invoice_partial_id uuid;
    v_payment1_id uuid;
BEGIN
    SELECT id INTO v_prop_id FROM properties WHERE slug = 'teren-test-hotel';
    SELECT id INTO v_user_id FROM users LIMIT 1;

    -- Booking 1: reserva checked_in de la habitación 101 (Juan Pérez)
    SELECT b.id INTO v_booking_101_id
    FROM bookings b
    JOIN guests g ON g.id = b.guest_id
    WHERE b.property_id = v_prop_id
      AND g.full_name = 'Juan Pérez'
    LIMIT 1;

    -- Booking 2: reserva confirmed de la habitación 102 (Maria Garcia)
    SELECT b.id INTO v_booking_102_id
    FROM bookings b
    JOIN guests g ON g.id = b.guest_id
    WHERE b.property_id = v_prop_id
      AND g.full_name = 'Maria Garcia'
    LIMIT 1;

    IF v_booking_101_id IS NULL OR v_booking_102_id IS NULL THEN
        RAISE NOTICE 'Skipping invoicing seed: bookings from 002_seed_bookings.sql not found';
        RETURN;
    END IF;

    -- ============================================================
    -- Invoice 1: PAGADA (booking 101)
    --   - subtotal: 500.000 IDR
    --   - PPN 11%: 55.000 IDR
    --   - total: 555.000 IDR
    --   - paid_at: ahora (total cubierto con un único pago QRIS)
    -- ============================================================
    INSERT INTO invoices (
        property_id, booking_id, invoice_number,
        subtotal, tax_amount, ppn_rate_snapshot, total,
        original_currency, exchange_rate,
        status, issued_at, paid_at, created_by
    ) VALUES (
        v_prop_id, v_booking_101_id,
        'INV-' || EXTRACT(YEAR FROM NOW()) || '-0001',
        500000.00, 55000.00, 0.1100, 555000.00,
        'IDR', 1.000000,
        'active', NOW(), NOW(), v_user_id
    )
    ON CONFLICT (property_id, invoice_number) DO NOTHING
    RETURNING id INTO v_invoice_paid_id;

    IF v_invoice_paid_id IS NOT NULL THEN
        INSERT INTO invoice_line_items (invoice_id, description, quantity, unit_price, total, sort_order)
        VALUES
            (v_invoice_paid_id, 'Room 101 - 3 nights', 3, 166666.67, 500000.00, 0);

        INSERT INTO payments (
            invoice_id, property_id, method, amount,
            original_currency, exchange_rate,
            reference, received_by
        ) VALUES (
            v_invoice_paid_id, v_prop_id, 'qris', 555000.00,
            'IDR', 1.000000,
            'QR-DEV-001', v_user_id
        )
        RETURNING id INTO v_payment1_id;
    END IF;

    -- ============================================================
    -- Invoice 2: PARCIAL (booking 102)
    --   - subtotal: 600.000 IDR
    --   - PPN 11%: 66.000 IDR
    --   - total: 666.000 IDR
    --   - paid: 300.000 IDR (depósito bank_transfer)
    --   - balance: 366.000 IDR
    -- ============================================================
    INSERT INTO invoices (
        property_id, booking_id, invoice_number,
        subtotal, tax_amount, ppn_rate_snapshot, total,
        original_currency, exchange_rate,
        status, issued_at, created_by
    ) VALUES (
        v_prop_id, v_booking_102_id,
        'INV-' || EXTRACT(YEAR FROM NOW()) || '-0002',
        600000.00, 66000.00, 0.1100, 666000.00,
        'IDR', 1.000000,
        'active', NOW(), v_user_id
    )
    ON CONFLICT (property_id, invoice_number) DO NOTHING
    RETURNING id INTO v_invoice_partial_id;

    IF v_invoice_partial_id IS NOT NULL THEN
        INSERT INTO invoice_line_items (invoice_id, description, quantity, unit_price, total, sort_order)
        VALUES
            (v_invoice_partial_id, 'Room 102 - 3 nights', 3, 200000.00, 600000.00, 0);

        INSERT INTO payments (
            invoice_id, property_id, method, amount,
            original_currency, exchange_rate,
            reference, notes, received_by
        ) VALUES (
            v_invoice_partial_id, v_prop_id, 'bank_transfer', 300000.00,
            'IDR', 1.000000,
            'TRF-DEV-002',
            'Depósito inicial. Saldo se cobra en check-in.',
            v_user_id
        );
    END IF;

    RAISE NOTICE 'Invoicing seed complete: 2 invoices, 2 line items, 2 payments';
END
$$;

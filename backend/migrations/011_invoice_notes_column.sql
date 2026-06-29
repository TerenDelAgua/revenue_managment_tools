-- 011_invoice_notes_column.sql
-- Add the `notes` column to invoices if missing. Idempotent.
-- requires: invoices.id
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS notes TEXT;
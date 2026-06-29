-- 003_combined.sql
-- requires: schema_migrations.filename, invoices.status
-- Mutates both schema_migrations (filename) and invoices (status) so the
-- header lists every precondition explicitly.
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS filename TEXT;

ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('active','void','refunded'));

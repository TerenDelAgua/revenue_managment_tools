-- 004_malformed.sql
-- requires: invoices_no_column
-- The value has no table.column separator and must fail parsing.
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS noop_check;

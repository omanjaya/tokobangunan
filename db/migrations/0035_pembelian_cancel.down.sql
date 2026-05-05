-- 0035_pembelian_cancel.down.sql
DROP INDEX IF EXISTS idx_pembelian_canceled;

ALTER TABLE pembelian
    DROP COLUMN IF EXISTS cancel_reason,
    DROP COLUMN IF EXISTS canceled_by,
    DROP COLUMN IF EXISTS canceled_at;

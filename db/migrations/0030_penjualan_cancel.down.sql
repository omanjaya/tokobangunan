-- 0030_penjualan_cancel.down.sql
DROP INDEX IF EXISTS idx_penjualan_canceled;
ALTER TABLE penjualan
    DROP COLUMN IF EXISTS cancel_reason,
    DROP COLUMN IF EXISTS canceled_by,
    DROP COLUMN IF EXISTS canceled_at;

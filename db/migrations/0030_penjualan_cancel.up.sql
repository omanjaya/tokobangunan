-- 0030_penjualan_cancel.up.sql
-- Tambah kolom cancel metadata pada partitioned table `penjualan`.
-- Kolom otomatis ter-propagate ke seluruh partisi (penjualan_2020..2030).

ALTER TABLE penjualan
    ADD COLUMN IF NOT EXISTS canceled_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS canceled_by    BIGINT REFERENCES "user"(id),
    ADD COLUMN IF NOT EXISTS cancel_reason  TEXT;

CREATE INDEX IF NOT EXISTS idx_penjualan_canceled
    ON penjualan(canceled_at)
    WHERE canceled_at IS NOT NULL;

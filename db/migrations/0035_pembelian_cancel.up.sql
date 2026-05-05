-- 0035_pembelian_cancel.up.sql
-- Tambah kolom cancel metadata pada tabel `pembelian` (non-partitioned).
-- Idempotent: ADD COLUMN IF NOT EXISTS.

ALTER TABLE pembelian
    ADD COLUMN IF NOT EXISTS canceled_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS canceled_by    BIGINT REFERENCES "user"(id),
    ADD COLUMN IF NOT EXISTS cancel_reason  TEXT;

CREATE INDEX IF NOT EXISTS idx_pembelian_canceled
    ON pembelian(canceled_at)
    WHERE canceled_at IS NOT NULL;

-- 0026: kolom PPN di penjualan & pembelian.
-- Logic dihitung di handler/service:
--   dpp        = subtotal - diskon
--   ppn_amount = dpp * ppn_persen / 100
--   total      = dpp + ppn_amount
-- ppn_persen=0 berarti tidak kena PPN (nilai default).

ALTER TABLE penjualan
    ADD COLUMN IF NOT EXISTS ppn_persen NUMERIC(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ppn_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dpp BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pembelian
    ADD COLUMN IF NOT EXISTS ppn_persen NUMERIC(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ppn_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dpp BIGINT NOT NULL DEFAULT 0;

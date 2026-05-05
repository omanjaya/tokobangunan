ALTER TABLE penjualan
    DROP COLUMN IF EXISTS ppn_persen,
    DROP COLUMN IF EXISTS ppn_amount,
    DROP COLUMN IF EXISTS dpp;

ALTER TABLE pembelian
    DROP COLUMN IF EXISTS ppn_persen,
    DROP COLUMN IF EXISTS ppn_amount,
    DROP COLUMN IF EXISTS dpp;

-- Tambah kolom diskon per item (cents). Default 0 untuk backward compat.
ALTER TABLE penjualan_item
    ADD COLUMN IF NOT EXISTS diskon BIGINT NOT NULL DEFAULT 0;

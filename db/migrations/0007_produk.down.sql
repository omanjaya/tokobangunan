DROP TRIGGER IF EXISTS trg_produk_updated ON produk;
DROP INDEX IF EXISTS idx_produk_active;
DROP INDEX IF EXISTS idx_produk_nama_trgm;
DROP TABLE IF EXISTS produk;

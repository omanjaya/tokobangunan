-- Drop unused trigram indexes (audit pg_stat_user_indexes idx_scan=0).
-- Total ~648 KB freed. Dataset kecil (produk 521, mitra 781) -> seq scan acceptable.
-- NOTE: jika dataset > 10K rows, restore index pakai:
--   CREATE INDEX <name> ON <table> USING gin (<col> gin_trgm_ops);

DROP INDEX IF EXISTS idx_mitra_nama_trgm;
DROP INDEX IF EXISTS idx_mitra_kode_trgm;
DROP INDEX IF EXISTS idx_produk_nama_trgm;
DROP INDEX IF EXISTS idx_supplier_nama_trgm;
DROP INDEX IF EXISTS idx_supplier_kode_trgm;
DROP INDEX IF EXISTS idx_user_username_trgm;
DROP INDEX IF EXISTS idx_pembelian_nomor_trgm;

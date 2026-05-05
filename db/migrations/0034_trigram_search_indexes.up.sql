-- Trigram GIN indexes untuk speed up ILIKE '%foo%' search.
-- pg_trgm extension sudah aktif sejak migrasi 0001/0008 (mitra.nama_trgm pakai gin_trgm_ops).
-- mitra.nama sudah punya trgm idx (idx_mitra_nama_trgm), skip.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_penjualan_nomor_trgm ON penjualan USING gin (nomor_kwitansi gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_pembelian_nomor_trgm ON pembelian USING gin (nomor_pembelian gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_mitra_kode_trgm      ON mitra     USING gin (kode gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_supplier_kode_trgm   ON supplier  USING gin (kode gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_supplier_nama_trgm   ON supplier  USING gin (nama gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_user_username_trgm   ON "user"    USING gin (username gin_trgm_ops);

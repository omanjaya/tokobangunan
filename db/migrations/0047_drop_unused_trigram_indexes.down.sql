-- Restore trigram indexes (rollback only; dataset masih kecil).
CREATE INDEX IF NOT EXISTS idx_mitra_nama_trgm ON mitra USING gin (nama gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_mitra_kode_trgm ON mitra USING gin (kode gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_produk_nama_trgm ON produk USING gin (nama gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_supplier_nama_trgm ON supplier USING gin (nama gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_supplier_kode_trgm ON supplier USING gin (kode gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_user_username_trgm ON "user" USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_pembelian_nomor_trgm ON pembelian USING gin (nomor gin_trgm_ops);

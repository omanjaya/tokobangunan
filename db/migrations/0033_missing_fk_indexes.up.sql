-- Tambah indeks untuk FK columns yang belum ter-index.
-- Hanya kolom yang benar-benar missing yang ditambah (sudah dicek via \d).
-- Skipped (sudah ada): pembelian_item.produk_id, mutasi_item.produk_id,
--   pembayaran.penjualan_id (covered by composite idx_pembayaran_penjualan),
--   audit_log.user_id (idx_audit_user), audit_log.tabel/record_id (idx_audit_table_record).

CREATE INDEX IF NOT EXISTS idx_penjualan_item_satuan ON penjualan_item (satuan_id);
CREATE INDEX IF NOT EXISTS idx_pembelian_item_satuan ON pembelian_item (satuan_id);
CREATE INDEX IF NOT EXISTS idx_mutasi_item_satuan    ON mutasi_item    (satuan_id);
CREATE INDEX IF NOT EXISTS idx_pembayaran_user       ON pembayaran     (user_id);

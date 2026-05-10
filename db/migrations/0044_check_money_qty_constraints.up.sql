-- 0044: CHECK constraints for money/qty hardening.
--
-- Money / total fields must be >= 0; qty fields must be > 0.
-- pembayaran.jumlah allows negative (refund/retur) but cannot be zero.
-- mutasi_item already has qty > 0 (legacy constraint); skipped.
-- tabungan_mitra already has debit/kredit >= 0 (legacy constraint); skipped.

-- penjualan
ALTER TABLE penjualan ADD CONSTRAINT chk_penjualan_total_nonneg    CHECK (total    >= 0);
ALTER TABLE penjualan ADD CONSTRAINT chk_penjualan_subtotal_nonneg CHECK (subtotal >= 0);
ALTER TABLE penjualan ADD CONSTRAINT chk_penjualan_diskon_nonneg   CHECK (diskon   >= 0);
ALTER TABLE penjualan ADD CONSTRAINT chk_penjualan_dpp_nonneg      CHECK (dpp      >= 0);
ALTER TABLE penjualan ADD CONSTRAINT chk_penjualan_ppn_nonneg      CHECK (ppn_amount >= 0);

-- penjualan_item
ALTER TABLE penjualan_item ADD CONSTRAINT chk_pjitem_qty_pos        CHECK (qty          > 0);
ALTER TABLE penjualan_item ADD CONSTRAINT chk_pjitem_konversi_pos   CHECK (qty_konversi > 0);
ALTER TABLE penjualan_item ADD CONSTRAINT chk_pjitem_harga_nonneg   CHECK (harga_satuan >= 0);
ALTER TABLE penjualan_item ADD CONSTRAINT chk_pjitem_subtotal_nonneg CHECK (subtotal    >= 0);
ALTER TABLE penjualan_item ADD CONSTRAINT chk_pjitem_diskon_nonneg  CHECK (diskon       >= 0);

-- pembelian
ALTER TABLE pembelian ADD CONSTRAINT chk_pembelian_total_nonneg    CHECK (total    >= 0);
ALTER TABLE pembelian ADD CONSTRAINT chk_pembelian_subtotal_nonneg CHECK (subtotal >= 0);

-- pembelian_item
ALTER TABLE pembelian_item ADD CONSTRAINT chk_pbitem_qty_pos      CHECK (qty          > 0);
ALTER TABLE pembelian_item ADD CONSTRAINT chk_pbitem_harga_nonneg CHECK (harga_satuan >= 0);

-- pembayaran (allow negative for retur, but not zero)
ALTER TABLE pembayaran ADD CONSTRAINT chk_pembayaran_jumlah_nonzero CHECK (jumlah <> 0);

-- harga_produk
ALTER TABLE harga_produk ADD CONSTRAINT chk_harga_jual_pos CHECK (harga_jual > 0);

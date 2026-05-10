-- 0044 down

ALTER TABLE penjualan DROP CONSTRAINT IF EXISTS chk_penjualan_total_nonneg;
ALTER TABLE penjualan DROP CONSTRAINT IF EXISTS chk_penjualan_subtotal_nonneg;
ALTER TABLE penjualan DROP CONSTRAINT IF EXISTS chk_penjualan_diskon_nonneg;
ALTER TABLE penjualan DROP CONSTRAINT IF EXISTS chk_penjualan_dpp_nonneg;
ALTER TABLE penjualan DROP CONSTRAINT IF EXISTS chk_penjualan_ppn_nonneg;

ALTER TABLE penjualan_item DROP CONSTRAINT IF EXISTS chk_pjitem_qty_pos;
ALTER TABLE penjualan_item DROP CONSTRAINT IF EXISTS chk_pjitem_konversi_pos;
ALTER TABLE penjualan_item DROP CONSTRAINT IF EXISTS chk_pjitem_harga_nonneg;
ALTER TABLE penjualan_item DROP CONSTRAINT IF EXISTS chk_pjitem_subtotal_nonneg;
ALTER TABLE penjualan_item DROP CONSTRAINT IF EXISTS chk_pjitem_diskon_nonneg;

ALTER TABLE pembelian DROP CONSTRAINT IF EXISTS chk_pembelian_total_nonneg;
ALTER TABLE pembelian DROP CONSTRAINT IF EXISTS chk_pembelian_subtotal_nonneg;

ALTER TABLE pembelian_item DROP CONSTRAINT IF EXISTS chk_pbitem_qty_pos;
ALTER TABLE pembelian_item DROP CONSTRAINT IF EXISTS chk_pbitem_harga_nonneg;

ALTER TABLE pembayaran DROP CONSTRAINT IF EXISTS chk_pembayaran_jumlah_nonzero;

ALTER TABLE harga_produk DROP CONSTRAINT IF EXISTS chk_harga_jual_pos;

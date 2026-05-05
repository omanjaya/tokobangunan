-- Drop duplicate btree indexes pada `penjualan`.
--
-- Audit menemukan dua btree indeks yang redundant dengan UNIQUE constraint
-- yang sudah ada (UNIQUE constraint sudah cukup sebagai indeks lookup):
--
--   - idx_penjualan_nomor (btree nomor_kwitansi)
--     redundant dengan uq_penjualan_nomor (UNIQUE btree nomor_kwitansi, tanggal).
--     Leading column = nomor_kwitansi -> uq_penjualan_nomor sudah cover semua
--     pencarian by nomor_kwitansi.
--
--   - idx_penjualan_id (btree id)
--     redundant dengan penjualan_pkey (UNIQUE btree id, tanggal).
--     Leading column = id -> pkey sudah cover lookup by id.
--
-- DROP INDEX pada parent (ONLY) di-propagate otomatis ke semua partisi child.
-- Trigram index (idx_penjualan_nomor_trgm, gin nomor_kwitansi gin_trgm_ops)
-- TIDAK di-drop -- beda op class, dipakai untuk ILIKE/% substring search.

DROP INDEX IF EXISTS idx_penjualan_nomor;
DROP INDEX IF EXISTS idx_penjualan_id;

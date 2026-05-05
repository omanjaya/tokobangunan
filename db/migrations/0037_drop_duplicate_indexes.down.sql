-- Re-create duplicate btree indexes (rollback dari 0037).
CREATE INDEX IF NOT EXISTS idx_penjualan_nomor ON penjualan USING btree (nomor_kwitansi);
CREATE INDEX IF NOT EXISTS idx_penjualan_id ON penjualan USING btree (id);

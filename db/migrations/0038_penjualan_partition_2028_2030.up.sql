-- Tambah partisi penjualan 2028-2030 (runway 5 tahun ke depan).
-- Catatan: 0032 sudah membuat partisi 2031-2040 + DEFAULT. Migrasi ini
-- mengisi celah 2028-2030 supaya runway kontigu tanpa fallback ke default.

CREATE TABLE IF NOT EXISTS penjualan_2028 PARTITION OF penjualan FOR VALUES FROM ('2028-01-01') TO ('2029-01-01');
CREATE TABLE IF NOT EXISTS penjualan_2029 PARTITION OF penjualan FOR VALUES FROM ('2029-01-01') TO ('2030-01-01');
CREATE TABLE IF NOT EXISTS penjualan_2030 PARTITION OF penjualan FOR VALUES FROM ('2030-01-01') TO ('2031-01-01');

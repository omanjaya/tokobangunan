-- Tambah partisi penjualan untuk tahun 2027-2030.
-- Default partition tabel `penjualan` di migrasi 0012 hanya 2025-2026.
-- Tambahkan untuk hindari INSERT failure ketika tahun berjalan ganti.

CREATE TABLE IF NOT EXISTS penjualan_2027 PARTITION OF penjualan
    FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2028 PARTITION OF penjualan
    FOR VALUES FROM ('2028-01-01') TO ('2029-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2029 PARTITION OF penjualan
    FOR VALUES FROM ('2029-01-01') TO ('2030-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2030 PARTITION OF penjualan
    FOR VALUES FROM ('2030-01-01') TO ('2031-01-01');

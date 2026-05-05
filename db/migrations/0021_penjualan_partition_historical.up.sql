-- Partisi untuk data historical (2020-2024) sebelum sistem live.
-- Excel asli punya transaksi 2022-2024; migrasi data perlu partition ini.

CREATE TABLE IF NOT EXISTS penjualan_2020 PARTITION OF penjualan
    FOR VALUES FROM ('2020-01-01') TO ('2021-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2021 PARTITION OF penjualan
    FOR VALUES FROM ('2021-01-01') TO ('2022-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2022 PARTITION OF penjualan
    FOR VALUES FROM ('2022-01-01') TO ('2023-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2023 PARTITION OF penjualan
    FOR VALUES FROM ('2023-01-01') TO ('2024-01-01');

CREATE TABLE IF NOT EXISTS penjualan_2024 PARTITION OF penjualan
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');

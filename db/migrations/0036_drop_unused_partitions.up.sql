-- Drop empty future partitions (2028-2040 + default catch-all).
-- Sebelumnya 22 partisi (2020-2040 + default) bikin planning overhead ~20ms
-- per query ke `penjualan` (Append node + partition pruning planning).
-- Strategy: keep partisi sampai 2027 saja (1 tahun runway). Saat dekat 2027,
-- bikin partisi baru manual (atau pakai pg_partman/cron job).
-- Semua partisi yang di-drop dipastikan kosong (count = 0).
--
-- Note: child partition kena referenced FK dari penjualan_item & pembayaran
-- via parent. Detach dulu dari parent supaya FK constraint dependency hilang,
-- baru bisa DROP TABLE.

ALTER TABLE penjualan DETACH PARTITION penjualan_default;
ALTER TABLE penjualan DETACH PARTITION penjualan_2028;
ALTER TABLE penjualan DETACH PARTITION penjualan_2029;
ALTER TABLE penjualan DETACH PARTITION penjualan_2030;
ALTER TABLE penjualan DETACH PARTITION penjualan_2031;
ALTER TABLE penjualan DETACH PARTITION penjualan_2032;
ALTER TABLE penjualan DETACH PARTITION penjualan_2033;
ALTER TABLE penjualan DETACH PARTITION penjualan_2034;
ALTER TABLE penjualan DETACH PARTITION penjualan_2035;
ALTER TABLE penjualan DETACH PARTITION penjualan_2036;
ALTER TABLE penjualan DETACH PARTITION penjualan_2037;
ALTER TABLE penjualan DETACH PARTITION penjualan_2038;
ALTER TABLE penjualan DETACH PARTITION penjualan_2039;
ALTER TABLE penjualan DETACH PARTITION penjualan_2040;

DROP TABLE IF EXISTS penjualan_default;
DROP TABLE IF EXISTS penjualan_2028;
DROP TABLE IF EXISTS penjualan_2029;
DROP TABLE IF EXISTS penjualan_2030;
DROP TABLE IF EXISTS penjualan_2031;
DROP TABLE IF EXISTS penjualan_2032;
DROP TABLE IF EXISTS penjualan_2033;
DROP TABLE IF EXISTS penjualan_2034;
DROP TABLE IF EXISTS penjualan_2035;
DROP TABLE IF EXISTS penjualan_2036;
DROP TABLE IF EXISTS penjualan_2037;
DROP TABLE IF EXISTS penjualan_2038;
DROP TABLE IF EXISTS penjualan_2039;
DROP TABLE IF EXISTS penjualan_2040;

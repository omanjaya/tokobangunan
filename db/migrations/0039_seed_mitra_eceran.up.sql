-- Seed default mitra "Eceran (Walk-in)" untuk POS tanpa pelanggan terdaftar.
-- Idempotent: skip jika kode/nama sudah ada.
INSERT INTO mitra (kode, nama, tipe, gudang_default_id, is_active, alamat, kontak)
SELECT 'ECERAN', 'Eceran (Walk-in)', 'eceran', NULL, TRUE, '', ''
WHERE NOT EXISTS (
    SELECT 1 FROM mitra
    WHERE kode = 'ECERAN' OR LOWER(nama) = 'eceran (walk-in)'
);

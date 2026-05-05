-- 0026_produk_foto.up.sql
-- Tambah kolom foto_url untuk URL gambar produk (relative path ke static).
ALTER TABLE produk ADD COLUMN IF NOT EXISTS foto_url TEXT;

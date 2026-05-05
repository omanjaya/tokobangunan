DROP TRIGGER IF EXISTS trg_produk_version       ON produk;
DROP TRIGGER IF EXISTS trg_mitra_version        ON mitra;
DROP TRIGGER IF EXISTS trg_supplier_version     ON supplier;
DROP TRIGGER IF EXISTS trg_gudang_version       ON gudang;
DROP TRIGGER IF EXISTS trg_harga_produk_version ON harga_produk;
DROP TRIGGER IF EXISTS trg_user_version         ON "user";

DROP FUNCTION IF EXISTS bump_version();

ALTER TABLE produk       DROP COLUMN IF EXISTS version;
ALTER TABLE mitra        DROP COLUMN IF EXISTS version;
ALTER TABLE supplier     DROP COLUMN IF EXISTS version;
ALTER TABLE gudang       DROP COLUMN IF EXISTS version;
ALTER TABLE harga_produk DROP COLUMN IF EXISTS version;
ALTER TABLE "user"       DROP COLUMN IF EXISTS version;

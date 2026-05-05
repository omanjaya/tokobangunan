-- Optimistic concurrency: kolom version + trigger bump_version untuk tabel master.
ALTER TABLE produk       ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE mitra        ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE supplier     ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE gudang       ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE harga_produk ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE "user"       ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;

CREATE OR REPLACE FUNCTION bump_version() RETURNS TRIGGER AS $$
BEGIN
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_produk_version       ON produk;
DROP TRIGGER IF EXISTS trg_mitra_version        ON mitra;
DROP TRIGGER IF EXISTS trg_supplier_version     ON supplier;
DROP TRIGGER IF EXISTS trg_gudang_version       ON gudang;
DROP TRIGGER IF EXISTS trg_harga_produk_version ON harga_produk;
DROP TRIGGER IF EXISTS trg_user_version         ON "user";

CREATE TRIGGER trg_produk_version       BEFORE UPDATE ON produk       FOR EACH ROW EXECUTE FUNCTION bump_version();
CREATE TRIGGER trg_mitra_version        BEFORE UPDATE ON mitra        FOR EACH ROW EXECUTE FUNCTION bump_version();
CREATE TRIGGER trg_supplier_version     BEFORE UPDATE ON supplier     FOR EACH ROW EXECUTE FUNCTION bump_version();
CREATE TRIGGER trg_gudang_version       BEFORE UPDATE ON gudang       FOR EACH ROW EXECUTE FUNCTION bump_version();
CREATE TRIGGER trg_harga_produk_version BEFORE UPDATE ON harga_produk FOR EACH ROW EXECUTE FUNCTION bump_version();
CREATE TRIGGER trg_user_version         BEFORE UPDATE ON "user"       FOR EACH ROW EXECUTE FUNCTION bump_version();

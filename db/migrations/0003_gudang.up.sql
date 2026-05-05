-- Tabel master gudang/cabang. 5 cabang awal: CANGGU, SAYAN, PEJENG, SAMPLANGAN, TEGES.
CREATE TABLE gudang (
    id          BIGSERIAL PRIMARY KEY,
    kode        TEXT NOT NULL UNIQUE,
    nama        TEXT NOT NULL,
    alamat      TEXT,
    telepon     TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_gudang_updated BEFORE UPDATE ON gudang
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

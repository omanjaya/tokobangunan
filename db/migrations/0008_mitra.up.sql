-- Tabel master mitra (customer). Tipe: eceran, grosir, proyek.
-- limit_kredit dalam cents (0 = tanpa limit).
CREATE TABLE mitra (
    id                  BIGSERIAL PRIMARY KEY,
    kode                TEXT NOT NULL UNIQUE,
    nama                TEXT NOT NULL,
    alamat              TEXT,
    kontak              TEXT,
    npwp                TEXT,
    tipe                TEXT NOT NULL,
    limit_kredit        BIGINT NOT NULL DEFAULT 0,
    jatuh_tempo_hari    INTEGER NOT NULL DEFAULT 30,
    gudang_default_id   BIGINT REFERENCES gudang(id),
    catatan             TEXT,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at          TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mitra_nama_trgm ON mitra USING gin (nama gin_trgm_ops);
CREATE INDEX idx_mitra_active ON mitra(is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_mitra_updated BEFORE UPDATE ON mitra
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

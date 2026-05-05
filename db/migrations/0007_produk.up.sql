-- Tabel master produk. faktor_konversi: 1 satuan_besar = N satuan_kecil.
-- GIN trigram index untuk autocomplete by nama.
CREATE TABLE produk (
    id                  BIGSERIAL PRIMARY KEY,
    sku                 TEXT NOT NULL UNIQUE,
    nama                TEXT NOT NULL,
    kategori            TEXT,
    satuan_kecil_id     BIGINT NOT NULL REFERENCES satuan(id),
    satuan_besar_id     BIGINT REFERENCES satuan(id),
    faktor_konversi     NUMERIC(12, 4) NOT NULL DEFAULT 1,
    stok_minimum        NUMERIC(14, 4) NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at          TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_produk_nama_trgm ON produk USING gin (nama gin_trgm_ops);
CREATE INDEX idx_produk_active ON produk(is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_produk_updated BEFORE UPDATE ON produk
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

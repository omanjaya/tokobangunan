-- Tabel master supplier (vendor barang).
CREATE TABLE supplier (
    id          BIGSERIAL PRIMARY KEY,
    kode        TEXT NOT NULL UNIQUE,
    nama        TEXT NOT NULL,
    alamat      TEXT,
    kontak      TEXT,
    catatan     TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_supplier_updated BEFORE UPDATE ON supplier
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

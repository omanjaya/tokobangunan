-- Tabel master satuan unit (sak, kg, batang, m, m2, lusin, biji, roll, lembar).
-- Spec doc tidak mencantumkan updated_at, namun ditambahkan untuk konsistensi.
CREATE TABLE satuan (
    id          BIGSERIAL PRIMARY KEY,
    kode        TEXT NOT NULL UNIQUE,
    nama        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_satuan_updated BEFORE UPDATE ON satuan
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

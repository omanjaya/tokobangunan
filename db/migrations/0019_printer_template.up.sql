-- Printer template per gudang/cabang. Setiap cabang punya konfigurasi printer
-- dot matrix sendiri (koordinat presisi untuk kertas NCR pre-printed).
-- MVP: koordinat hardcoded di kode generator; tabel ini menyiapkan tempat
-- konfigurasi future ketika koordinat per-template diimplementasi.

CREATE TABLE printer_template (
    id            BIGSERIAL PRIMARY KEY,
    gudang_id     BIGINT NOT NULL REFERENCES gudang(id),
    jenis         TEXT NOT NULL,                -- kwitansi, struk, label
    nama          TEXT NOT NULL,
    lebar_char    INTEGER NOT NULL DEFAULT 80,
    panjang_baris INTEGER NOT NULL DEFAULT 33,
    offset_x      INTEGER NOT NULL DEFAULT 0,
    offset_y      INTEGER NOT NULL DEFAULT 0,
    koordinat     JSONB NOT NULL DEFAULT '{}'::jsonb,
    preview       TEXT,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (gudang_id, jenis, nama)
);

CREATE INDEX idx_printer_template_gudang ON printer_template(gudang_id);

CREATE TRIGGER trg_printer_template_updated BEFORE UPDATE ON printer_template
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

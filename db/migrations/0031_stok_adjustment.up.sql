-- Penyesuaian stok (single-step) + audit trail.
-- Setiap baris adalah satu adjustment qty pada (gudang, produk).
-- qty bisa negatif (rusak/hilang/sample) atau positif (initial/koreksi+/return).
CREATE TABLE IF NOT EXISTS stok_adjustment (
    id            BIGSERIAL PRIMARY KEY,
    gudang_id     BIGINT NOT NULL REFERENCES gudang(id),
    produk_id     BIGINT NOT NULL REFERENCES produk(id),
    satuan_id     BIGINT NOT NULL REFERENCES satuan(id),
    qty           NUMERIC(14,4) NOT NULL,
    qty_konversi  NUMERIC(14,4) NOT NULL,
    kategori      TEXT NOT NULL,
    alasan        TEXT NOT NULL,
    catatan       TEXT,
    user_id       BIGINT NOT NULL REFERENCES "user"(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_stok_adj_kategori CHECK (kategori IN (
        'initial','koreksi','rusak','hilang','sample','hadiah',
        'return_supplier','return_customer'
    )),
    CONSTRAINT chk_stok_adj_qty_nonzero CHECK (qty <> 0)
);

CREATE INDEX IF NOT EXISTS idx_stok_adj_gudang_produk
    ON stok_adjustment(gudang_id, produk_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stok_adj_kategori
    ON stok_adjustment(kategori, created_at DESC);

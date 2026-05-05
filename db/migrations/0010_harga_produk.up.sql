-- History harga jual per produk per gudang per tipe (eceran/grosir/proyek).
-- gudang_id NULL = berlaku untuk semua gudang. harga_jual dalam cents.
CREATE TABLE harga_produk (
    id              BIGSERIAL PRIMARY KEY,
    produk_id       BIGINT NOT NULL REFERENCES produk(id),
    gudang_id       BIGINT NULL REFERENCES gudang(id),
    tipe            TEXT NOT NULL,
    harga_jual      BIGINT NOT NULL,
    berlaku_dari    DATE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (produk_id, gudang_id, tipe, berlaku_dari)
);

CREATE INDEX idx_harga_lookup ON harga_produk(produk_id, gudang_id, tipe, berlaku_dari DESC);

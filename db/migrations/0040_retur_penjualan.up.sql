-- Retur Penjualan: customer mengembalikan barang dari invoice penjualan.
-- Stok bertambah, dan refund dicatat sebagai pembayaran negatif.

CREATE TABLE IF NOT EXISTS retur_penjualan (
    id BIGSERIAL PRIMARY KEY,
    nomor_retur TEXT NOT NULL UNIQUE,
    penjualan_id BIGINT NOT NULL,
    penjualan_tanggal DATE NOT NULL,
    mitra_id BIGINT REFERENCES mitra(id),
    gudang_id BIGINT NOT NULL REFERENCES gudang(id),
    tanggal DATE NOT NULL,
    alasan TEXT NOT NULL,
    catatan TEXT,
    subtotal_refund BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (penjualan_id, penjualan_tanggal) REFERENCES penjualan(id, tanggal)
);

CREATE TABLE IF NOT EXISTS retur_penjualan_item (
    id BIGSERIAL PRIMARY KEY,
    retur_id BIGINT NOT NULL REFERENCES retur_penjualan(id) ON DELETE CASCADE,
    penjualan_item_id BIGINT NOT NULL REFERENCES penjualan_item(id),
    produk_id BIGINT NOT NULL REFERENCES produk(id),
    qty NUMERIC(14,4) NOT NULL CHECK (qty > 0),
    qty_konversi NUMERIC(14,4) NOT NULL,
    satuan_id BIGINT NOT NULL REFERENCES satuan(id),
    harga_satuan BIGINT NOT NULL,
    subtotal BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_retur_penjualan_inv ON retur_penjualan(penjualan_id, penjualan_tanggal);
CREATE INDEX IF NOT EXISTS idx_retur_penjualan_mitra ON retur_penjualan(mitra_id, tanggal DESC);
CREATE INDEX IF NOT EXISTS idx_retur_item_retur ON retur_penjualan_item(retur_id);
CREATE INDEX IF NOT EXISTS idx_retur_item_penjualan_item ON retur_penjualan_item(penjualan_item_id);

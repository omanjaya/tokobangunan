-- Pembelian: tabel hutang supplier + item + pembayaran.
-- Berbeda dengan penjualan, pembelian TIDAK di-partition (volume jauh
-- lebih kecil dibanding penjualan harian).

CREATE TABLE pembelian (
    id              BIGSERIAL PRIMARY KEY,
    nomor_pembelian TEXT NOT NULL UNIQUE,
    tanggal         DATE NOT NULL,
    supplier_id     BIGINT NOT NULL REFERENCES supplier(id),
    gudang_id       BIGINT NOT NULL REFERENCES gudang(id),
    user_id         BIGINT NOT NULL REFERENCES "user"(id),
    subtotal        BIGINT NOT NULL,                 -- cents
    diskon          BIGINT NOT NULL DEFAULT 0,
    total           BIGINT NOT NULL,
    status_bayar    TEXT NOT NULL,                   -- lunas | kredit | sebagian
    jatuh_tempo     DATE,
    catatan         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pembelian_tanggal_gudang ON pembelian(tanggal, gudang_id);
CREATE INDEX idx_pembelian_supplier_status ON pembelian(supplier_id, status_bayar);
CREATE INDEX idx_pembelian_nomor          ON pembelian(nomor_pembelian);

CREATE TRIGGER trg_pembelian_updated BEFORE UPDATE ON pembelian
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pembelian_item (
    id              BIGSERIAL PRIMARY KEY,
    pembelian_id    BIGINT NOT NULL REFERENCES pembelian(id) ON DELETE CASCADE,
    produk_id       BIGINT NOT NULL REFERENCES produk(id),
    produk_nama     TEXT NOT NULL,                  -- snapshot saat pembelian
    qty             NUMERIC(14, 4) NOT NULL,
    satuan_id       BIGINT NOT NULL REFERENCES satuan(id),
    satuan_kode     TEXT NOT NULL,                  -- snapshot
    qty_konversi    NUMERIC(14, 4) NOT NULL,        -- qty dalam satuan kecil
    harga_satuan    BIGINT NOT NULL,                -- cents per satuan
    subtotal        BIGINT NOT NULL
);

CREATE INDEX idx_pembelian_item_pembelian ON pembelian_item(pembelian_id);
CREATE INDEX idx_pembelian_item_produk    ON pembelian_item(produk_id);

CREATE TABLE pembayaran_supplier (
    id              BIGSERIAL PRIMARY KEY,
    pembelian_id    BIGINT NULL REFERENCES pembelian(id),
    supplier_id     BIGINT NOT NULL REFERENCES supplier(id),
    tanggal         DATE NOT NULL,
    jumlah          BIGINT NOT NULL,                -- cents
    metode          TEXT NOT NULL,                  -- tunai | transfer | cek | giro
    referensi       TEXT,
    user_id         BIGINT NOT NULL REFERENCES "user"(id),
    catatan         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pembayaran_supplier_supplier ON pembayaran_supplier(supplier_id, tanggal);
CREATE INDEX idx_pembayaran_supplier_pembelian ON pembayaran_supplier(pembelian_id);

-- Trigger: tambah stok saat insert pembelian_item.
CREATE OR REPLACE FUNCTION update_stok_pembelian()
RETURNS TRIGGER AS $$
DECLARE
    v_gudang_id BIGINT;
BEGIN
    SELECT gudang_id INTO v_gudang_id FROM pembelian WHERE id = NEW.pembelian_id;

    UPDATE stok
    SET qty = qty + NEW.qty_konversi,
        updated_at = now()
    WHERE produk_id = NEW.produk_id
      AND gudang_id = v_gudang_id;

    IF NOT FOUND THEN
        INSERT INTO stok (gudang_id, produk_id, qty)
        VALUES (v_gudang_id, NEW.produk_id, NEW.qty_konversi);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_stok_pembelian AFTER INSERT ON pembelian_item
    FOR EACH ROW EXECUTE FUNCTION update_stok_pembelian();

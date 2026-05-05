-- Mutasi antar gudang. Header + line items.
-- client_uuid untuk idempotency dari form submission.
-- Status workflow: draft -> dikirim -> diterima. draft -> dibatalkan.
CREATE TABLE mutasi_gudang (
    id                  BIGSERIAL PRIMARY KEY,
    nomor_mutasi        TEXT NOT NULL UNIQUE,
    tanggal             DATE NOT NULL,
    gudang_asal_id      BIGINT NOT NULL REFERENCES gudang(id),
    gudang_tujuan_id    BIGINT NOT NULL REFERENCES gudang(id),
    status              TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft', 'dikirim', 'diterima', 'dibatalkan')),
    user_pengirim_id    BIGINT NULL REFERENCES "user"(id),
    user_penerima_id    BIGINT NULL REFERENCES "user"(id),
    tanggal_kirim       TIMESTAMPTZ NULL,
    tanggal_terima      TIMESTAMPTZ NULL,
    catatan             TEXT NOT NULL DEFAULT '',
    client_uuid         UUID NOT NULL UNIQUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (gudang_asal_id <> gudang_tujuan_id)
);

CREATE INDEX idx_mutasi_tanggal ON mutasi_gudang(tanggal DESC);
CREATE INDEX idx_mutasi_status ON mutasi_gudang(status);
CREATE INDEX idx_mutasi_asal ON mutasi_gudang(gudang_asal_id);
CREATE INDEX idx_mutasi_tujuan ON mutasi_gudang(gudang_tujuan_id);

CREATE TRIGGER trg_mutasi_gudang_updated BEFORE UPDATE ON mutasi_gudang
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Line item mutasi. produk_nama dan satuan_kode di-snapshot supaya history
-- tetap akurat walau master direname/satuan berubah. qty_konversi adalah qty
-- dalam satuan_kecil (untuk perhitungan stok).
CREATE TABLE mutasi_item (
    id              BIGSERIAL PRIMARY KEY,
    mutasi_id       BIGINT NOT NULL REFERENCES mutasi_gudang(id) ON DELETE CASCADE,
    produk_id       BIGINT NOT NULL REFERENCES produk(id),
    produk_nama     TEXT NOT NULL,
    qty             NUMERIC(14, 4) NOT NULL CHECK (qty > 0),
    satuan_id       BIGINT NOT NULL REFERENCES satuan(id),
    satuan_kode     TEXT NOT NULL,
    qty_konversi    NUMERIC(14, 4) NOT NULL CHECK (qty_konversi > 0),
    harga_internal  BIGINT NULL,
    catatan         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_mutasi_item_mutasi ON mutasi_item(mutasi_id);
CREATE INDEX idx_mutasi_item_produk ON mutasi_item(produk_id);

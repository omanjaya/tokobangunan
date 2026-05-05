-- Penjualan: tabel utama transaksi penjualan + item.
-- Tabel `penjualan` di-partition by RANGE (tanggal) per tahun untuk performance.
-- Karena partitioning, primary key wajib include partition key (tanggal).

CREATE TABLE penjualan (
    id              BIGSERIAL,
    nomor_kwitansi  TEXT NOT NULL,
    tanggal         DATE NOT NULL,
    mitra_id        BIGINT NOT NULL REFERENCES mitra(id),
    gudang_id       BIGINT NOT NULL REFERENCES gudang(id),
    user_id         BIGINT NOT NULL REFERENCES "user"(id),
    subtotal        BIGINT NOT NULL,                 -- cents
    diskon          BIGINT NOT NULL DEFAULT 0,
    total           BIGINT NOT NULL,
    status_bayar    TEXT NOT NULL,                   -- lunas | kredit | sebagian
    jatuh_tempo     DATE,
    catatan         TEXT,
    client_uuid     UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, tanggal)
) PARTITION BY RANGE (tanggal);

-- Partition awal (per tahun).
CREATE TABLE penjualan_2025 PARTITION OF penjualan
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE penjualan_2026 PARTITION OF penjualan
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- Indeks (otomatis ter-propagate ke partisi).
CREATE INDEX idx_penjualan_tanggal_gudang ON penjualan(tanggal, gudang_id);
CREATE INDEX idx_penjualan_mitra_status   ON penjualan(mitra_id, status_bayar);
CREATE INDEX idx_penjualan_nomor          ON penjualan(nomor_kwitansi);
CREATE INDEX idx_penjualan_id             ON penjualan(id);

-- UNIQUE harus include partition key di partitioned table.
CREATE UNIQUE INDEX uq_penjualan_nomor      ON penjualan(nomor_kwitansi, tanggal);
CREATE UNIQUE INDEX uq_penjualan_clientuuid ON penjualan(client_uuid, tanggal);

-- Item penjualan (composite FK ke (penjualan_id, penjualan_tanggal)).
CREATE TABLE penjualan_item (
    id                BIGSERIAL PRIMARY KEY,
    penjualan_id      BIGINT NOT NULL,
    penjualan_tanggal DATE NOT NULL,
    produk_id         BIGINT NOT NULL REFERENCES produk(id),
    produk_nama       TEXT NOT NULL,                 -- snapshot saat penjualan
    qty               NUMERIC(14, 4) NOT NULL,
    satuan_id         BIGINT NOT NULL REFERENCES satuan(id),
    satuan_kode       TEXT NOT NULL,                 -- snapshot
    qty_konversi      NUMERIC(14, 4) NOT NULL,       -- qty dalam satuan kecil
    harga_satuan      BIGINT NOT NULL,               -- cents per satuan
    subtotal          BIGINT NOT NULL,
    FOREIGN KEY (penjualan_id, penjualan_tanggal)
        REFERENCES penjualan(id, tanggal) ON DELETE CASCADE
);

CREATE INDEX idx_penjualan_item_penjualan ON penjualan_item(penjualan_id, penjualan_tanggal);
CREATE INDEX idx_penjualan_item_produk    ON penjualan_item(produk_id);

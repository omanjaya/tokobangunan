-- 0027: cashflow tracking (kas masuk/keluar non-penjualan/pembelian).
CREATE TABLE IF NOT EXISTS cashflow (
    id           BIGSERIAL PRIMARY KEY,
    nomor        TEXT NOT NULL UNIQUE,
    tanggal      DATE NOT NULL,
    gudang_id    BIGINT REFERENCES gudang(id),
    tipe         TEXT NOT NULL CHECK (tipe IN ('masuk', 'keluar')),
    kategori     TEXT NOT NULL,
    deskripsi    TEXT,
    jumlah       BIGINT NOT NULL CHECK (jumlah > 0),
    metode       TEXT NOT NULL,
    referensi    TEXT,
    user_id      BIGINT NOT NULL REFERENCES "user"(id),
    catatan      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cashflow_tanggal ON cashflow(tanggal);
CREATE INDEX IF NOT EXISTS idx_cashflow_gudang_tipe ON cashflow(gudang_id, tipe);

CREATE TABLE IF NOT EXISTS cashflow_kategori (
    id   BIGSERIAL PRIMARY KEY,
    nama TEXT NOT NULL UNIQUE,
    tipe TEXT NOT NULL CHECK (tipe IN ('masuk', 'keluar'))
);

INSERT INTO cashflow_kategori (nama, tipe) VALUES
    ('Setoran Modal',    'masuk'),
    ('Pendapatan Lain',  'masuk'),
    ('Sewa',             'keluar'),
    ('Listrik',          'keluar'),
    ('Air',              'keluar'),
    ('Gaji',             'keluar'),
    ('Transportasi',     'keluar'),
    ('Konsumsi',         'keluar'),
    ('ATK',              'keluar'),
    ('Maintenance',      'keluar'),
    ('Pajak',            'keluar'),
    ('Lain-lain',        'keluar')
ON CONFLICT (nama) DO NOTHING;

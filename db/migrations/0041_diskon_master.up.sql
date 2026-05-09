CREATE TABLE IF NOT EXISTS diskon_master (
    id BIGSERIAL PRIMARY KEY,
    kode TEXT NOT NULL UNIQUE,
    nama TEXT NOT NULL,
    tipe TEXT NOT NULL CHECK (tipe IN ('persen','nominal')),
    nilai NUMERIC(14,4) NOT NULL CHECK (nilai > 0),
    min_subtotal BIGINT NOT NULL DEFAULT 0,
    max_diskon BIGINT,
    berlaku_dari DATE NOT NULL,
    berlaku_sampai DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_diskon_active ON diskon_master(is_active, berlaku_dari, berlaku_sampai);

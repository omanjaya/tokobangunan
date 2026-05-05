-- Tabungan mitra: catatan setor (debit) & tarik (kredit) saldo titip.
-- Saldo running balance dihitung di service (lock terakhir SELECT FOR UPDATE),
-- bukan trigger, agar transparan dan mudah audit.

CREATE TABLE tabungan_mitra (
    id          BIGSERIAL PRIMARY KEY,
    mitra_id    BIGINT NOT NULL REFERENCES mitra(id),
    tanggal     DATE NOT NULL,
    debit       BIGINT NOT NULL DEFAULT 0,
    kredit      BIGINT NOT NULL DEFAULT 0,
    saldo       BIGINT NOT NULL,
    catatan     TEXT,
    user_id     BIGINT NOT NULL REFERENCES "user"(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (debit >= 0 AND kredit >= 0),
    CHECK (NOT (debit > 0 AND kredit > 0))
);

CREATE INDEX idx_tabungan_mitra_tanggal ON tabungan_mitra(mitra_id, tanggal);

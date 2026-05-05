-- Audit log untuk semua aksi CREATE/UPDATE/DELETE. Retensi 7 tahun (audit pajak).
CREATE TABLE audit_log (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES "user"(id),
    aksi            TEXT NOT NULL,
    tabel           TEXT NOT NULL,
    record_id       BIGINT NOT NULL,
    payload_before  JSONB,
    payload_after   JSONB,
    ip              INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_table_record ON audit_log(tabel, record_id);
CREATE INDEX idx_audit_user ON audit_log(user_id);
CREATE INDEX idx_audit_created ON audit_log(created_at);

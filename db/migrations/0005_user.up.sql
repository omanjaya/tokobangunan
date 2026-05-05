-- Tabel user (quoted karena reserved word). Role: owner, admin, kasir, gudang.
-- gudang_id NULL = akses semua cabang (owner/admin pusat).
CREATE TABLE "user" (
    id              BIGSERIAL PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    nama_lengkap    TEXT NOT NULL,
    email           TEXT,
    role            TEXT NOT NULL,
    gudang_id       BIGINT NULL REFERENCES gudang(id),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at   TIMESTAMPTZ NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_user_updated BEFORE UPDATE ON "user"
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 0047: tambah kolom request_id ke audit_log untuk korelasi dengan log access.
--
-- request_id berasal dari Echo RequestID middleware (header X-Request-Id).
-- Kolom nullable: row historis biarkan NULL.
--
-- Catatan: ALTER TABLE adalah DDL — tidak ter-trigger oleh BEFORE UPDATE
-- append-only enforcement (yang bekerja per-row UPDATE/DELETE/TRUNCATE).
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS request_id TEXT;

-- Index parsial: hanya rows yang punya request_id (mayoritas baru pasca-deploy).
CREATE INDEX IF NOT EXISTS idx_audit_request_id
    ON audit_log(request_id)
    WHERE request_id IS NOT NULL;

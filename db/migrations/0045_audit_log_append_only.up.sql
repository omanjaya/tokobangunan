-- 0045: audit_log append-only.
--
-- Cleanup leftover test rows from previous test runs first.
-- Then install BEFORE UPDATE/DELETE/TRUNCATE triggers that raise an exception.
-- Triggers fire for ALL roles (including owners). Privileged purge tasks
-- (e.g. retention) must temporarily DISABLE the triggers.

-- 1. Cleanup historical test rows BEFORE locking the table.
DELETE FROM audit_log WHERE tabel LIKE 'test_%' OR aksi LIKE 'test_%';

-- 2. Append-only enforcement function + triggers.
CREATE OR REPLACE FUNCTION audit_log_append_only()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only (% not allowed)', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

CREATE TRIGGER audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_append_only();

-- 3. Belt-and-suspenders: revoke mutation grants.
REVOKE UPDATE, DELETE, TRUNCATE ON audit_log FROM PUBLIC;

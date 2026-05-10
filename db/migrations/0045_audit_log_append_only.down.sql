-- 0045 down

DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_delete ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_truncate ON audit_log;
DROP FUNCTION IF EXISTS audit_log_append_only();

GRANT UPDATE, DELETE, TRUNCATE ON audit_log TO PUBLIC;

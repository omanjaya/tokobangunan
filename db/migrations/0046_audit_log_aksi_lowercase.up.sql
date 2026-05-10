-- 0046: normalize audit_log.aksi to lowercase.
--
-- Sebelumnya middleware menulis UPPERCASE (CREATE, UPDATE, ...) sementara
-- service-level audit menulis lowercase (create, update, ...). Inkonsistensi
-- ini bikin filter query `WHERE aksi = 'create'` tidak menangkap semua row.
--
-- Migration ini normalize semua row existing ke lowercase. Append-only
-- trigger (lihat 0045) di-disable sementara karena migration ini perlu
-- UPDATE row existing.
--
-- Idempotent: aman re-run (hanya update row yang masih uppercase).

ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_update;

UPDATE audit_log
SET aksi = LOWER(aksi)
WHERE aksi <> LOWER(aksi);

ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_update;

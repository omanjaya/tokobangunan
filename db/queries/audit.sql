-- name: InsertAuditLog :one
INSERT INTO audit_log (
    user_id, aksi, tabel, record_id,
    payload_before, payload_after, ip, user_agent
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: ListAuditByRecord :many
SELECT * FROM audit_log
WHERE tabel = $1 AND record_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetUserByUsername :one
SELECT * FROM "user" WHERE username = $1 AND is_active = true;

-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: CreateUser :one
INSERT INTO "user" (
    username, password_hash, nama_lengkap, email, role, gudang_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE "user" SET password_hash = $2 WHERE id = $1;

-- name: IncrementFailedAttempts :exec
UPDATE "user" SET failed_attempts = failed_attempts + 1 WHERE id = $1;

-- name: ResetFailedAttempts :exec
UPDATE "user" SET failed_attempts = 0, locked_until = NULL WHERE id = $1;

-- name: LockUser :exec
UPDATE "user" SET locked_until = $2 WHERE id = $1;

-- name: UpdateLastLogin :exec
UPDATE "user" SET last_login_at = now() WHERE id = $1;

-- name: CreateSession :one
INSERT INTO session (user_id, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM session WHERE id = $1 AND expires_at > now();

-- name: DeleteSession :exec
DELETE FROM session WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM session WHERE expires_at <= now();

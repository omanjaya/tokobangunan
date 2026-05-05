-- name: ListSatuan :many
SELECT * FROM satuan ORDER BY kode;

-- name: GetSatuanByKode :one
SELECT * FROM satuan WHERE kode = $1;

-- name: CreateSatuan :one
INSERT INTO satuan (kode, nama)
VALUES ($1, $2)
RETURNING *;

-- name: ListGudang :many
SELECT * FROM gudang WHERE is_active = true ORDER BY kode;

-- name: GetGudangByID :one
SELECT * FROM gudang WHERE id = $1;

-- name: GetGudangByKode :one
SELECT * FROM gudang WHERE kode = $1;

-- name: CreateGudang :one
INSERT INTO gudang (kode, nama, alamat, telepon)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateGudang :one
UPDATE gudang
SET nama = $2, alamat = $3, telepon = $4, is_active = $5
WHERE id = $1
RETURNING *;

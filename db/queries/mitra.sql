-- name: ListMitra :many
SELECT * FROM mitra
WHERE deleted_at IS NULL
  AND (sqlc.narg('tipe')::text IS NULL OR tipe = sqlc.narg('tipe')::text)
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY nama
LIMIT $1 OFFSET $2;

-- name: SearchMitra :many
SELECT * FROM mitra
WHERE deleted_at IS NULL
  AND is_active = true
  AND nama % sqlc.arg('query')::text
ORDER BY similarity(nama, sqlc.arg('query')::text) DESC
LIMIT $1;

-- name: GetMitraByID :one
SELECT * FROM mitra WHERE id = $1 AND deleted_at IS NULL;

-- name: GetMitraByKode :one
SELECT * FROM mitra WHERE kode = $1 AND deleted_at IS NULL;

-- name: CreateMitra :one
INSERT INTO mitra (
    kode, nama, alamat, kontak, npwp, tipe,
    limit_kredit, jatuh_tempo_hari, gudang_default_id, catatan
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: UpdateMitra :one
UPDATE mitra
SET nama = $2,
    alamat = $3,
    kontak = $4,
    npwp = $5,
    tipe = $6,
    limit_kredit = $7,
    jatuh_tempo_hari = $8,
    gudang_default_id = $9,
    catatan = $10,
    is_active = $11
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMitra :exec
UPDATE mitra SET deleted_at = now(), is_active = false WHERE id = $1;

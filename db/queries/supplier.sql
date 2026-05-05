-- name: ListSupplier :many
SELECT * FROM supplier
WHERE deleted_at IS NULL
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY nama
LIMIT $1 OFFSET $2;

-- name: GetSupplierByID :one
SELECT * FROM supplier WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateSupplier :one
INSERT INTO supplier (kode, nama, alamat, kontak, catatan)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateSupplier :one
UPDATE supplier
SET nama = $2,
    alamat = $3,
    kontak = $4,
    catatan = $5,
    is_active = $6
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

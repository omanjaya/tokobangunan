-- name: ListProduk :many
SELECT * FROM produk
WHERE deleted_at IS NULL
  AND (sqlc.narg('kategori')::text IS NULL OR kategori = sqlc.narg('kategori')::text)
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY nama
LIMIT $1 OFFSET $2;

-- name: SearchProduk :many
SELECT * FROM produk
WHERE deleted_at IS NULL
  AND is_active = true
  AND nama % sqlc.arg('query')::text
ORDER BY similarity(nama, sqlc.arg('query')::text) DESC
LIMIT $1;

-- name: GetProdukByID :one
SELECT * FROM produk WHERE id = $1 AND deleted_at IS NULL;

-- name: GetProdukBySKU :one
SELECT * FROM produk WHERE sku = $1 AND deleted_at IS NULL;

-- name: CreateProduk :one
INSERT INTO produk (
    sku, nama, kategori, satuan_kecil_id, satuan_besar_id,
    faktor_konversi, stok_minimum
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: UpdateProduk :one
UPDATE produk
SET nama = $2,
    kategori = $3,
    satuan_kecil_id = $4,
    satuan_besar_id = $5,
    faktor_konversi = $6,
    stok_minimum = $7,
    is_active = $8
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProduk :exec
UPDATE produk SET deleted_at = now(), is_active = false WHERE id = $1;

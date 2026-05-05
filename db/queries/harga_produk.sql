-- name: GetHargaAktif :one
SELECT * FROM harga_produk
WHERE produk_id = $1
  AND (gudang_id = sqlc.narg('gudang_id')::bigint OR gudang_id IS NULL)
  AND tipe = $2
  AND berlaku_dari <= CURRENT_DATE
ORDER BY gudang_id NULLS LAST, berlaku_dari DESC
LIMIT 1;

-- name: CreateHargaProduk :one
INSERT INTO harga_produk (produk_id, gudang_id, tipe, harga_jual, berlaku_dari)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListHargaByProduk :many
SELECT * FROM harga_produk
WHERE produk_id = $1
ORDER BY berlaku_dari DESC, gudang_id NULLS LAST;

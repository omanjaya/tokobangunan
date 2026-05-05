package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// HargaRepo akses tabel harga_produk.
type HargaRepo struct {
	pool *pgxpool.Pool
}

func NewHargaRepo(pool *pgxpool.Pool) *HargaRepo {
	return &HargaRepo{pool: pool}
}

const hargaColumns = `id, produk_id, gudang_id, tipe, harga_jual, berlaku_dari, created_at`

func scanHarga(row pgx.Row, h *domain.HargaProduk) error {
	return row.Scan(&h.ID, &h.ProdukID, &h.GudangID, &h.Tipe, &h.HargaJual,
		&h.BerlakuDari, &h.CreatedAt)
}

// GetAktif ambil baris terbaru dengan berlaku_dari <= today untuk
// (produk, gudang, tipe). gudangID nil cocok dengan baris yang gudang_id IS NULL.
func (r *HargaRepo) GetAktif(ctx context.Context, produkID int64, gudangID *int64, tipe string) (*domain.HargaProduk, error) {
	var sql string
	var args []any
	if gudangID == nil {
		sql = `SELECT ` + hargaColumns + ` FROM harga_produk
			WHERE produk_id = $1 AND gudang_id IS NULL AND tipe = $2
			  AND berlaku_dari <= CURRENT_DATE
			ORDER BY berlaku_dari DESC LIMIT 1`
		args = []any{produkID, tipe}
	} else {
		sql = `SELECT ` + hargaColumns + ` FROM harga_produk
			WHERE produk_id = $1 AND gudang_id = $2 AND tipe = $3
			  AND berlaku_dari <= CURRENT_DATE
			ORDER BY berlaku_dari DESC LIMIT 1`
		args = []any{produkID, *gudangID, tipe}
	}
	var h domain.HargaProduk
	if err := scanHarga(r.pool.QueryRow(ctx, sql, args...), &h); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHargaNotFound
		}
		return nil, fmt.Errorf("get harga aktif: %w", err)
	}
	return &h, nil
}

// Create insert baris baru ke history harga.
func (r *HargaRepo) Create(ctx context.Context, h *domain.HargaProduk) error {
	const sql = `INSERT INTO harga_produk
		(produk_id, gudang_id, tipe, harga_jual, berlaku_dari)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	row := r.pool.QueryRow(ctx, sql, h.ProdukID, h.GudangID, h.Tipe, h.HargaJual, h.BerlakuDari)
	if err := row.Scan(&h.ID, &h.CreatedAt); err != nil {
		return fmt.Errorf("create harga: %w", err)
	}
	return nil
}

// ListByProduk seluruh history harga untuk satu produk (terbaru dulu).
func (r *HargaRepo) ListByProduk(ctx context.Context, produkID int64) ([]domain.HargaProduk, error) {
	const sql = `SELECT ` + hargaColumns + ` FROM harga_produk
		WHERE produk_id = $1
		ORDER BY berlaku_dari DESC, tipe ASC, created_at DESC`
	rows, err := r.pool.Query(ctx, sql, produkID)
	if err != nil {
		return nil, fmt.Errorf("list harga: %w", err)
	}
	defer rows.Close()
	out := []domain.HargaProduk{}
	for rows.Next() {
		var h domain.HargaProduk
		if err := scanHarga(rows, &h); err != nil {
			return nil, fmt.Errorf("scan harga: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

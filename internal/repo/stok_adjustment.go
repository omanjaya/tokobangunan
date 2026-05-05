package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListAdjFilter filter listing penyesuaian stok.
type ListAdjFilter struct {
	GudangID *int64
	ProdukID *int64
	Kategori string
	From     *time.Time
	To       *time.Time
	Page     int
	PerPage  int
}

// Normalize set default page/perpage.
func (f *ListAdjFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
}

// AdjRepo akses tabel stok_adjustment.
type AdjRepo struct {
	pool *pgxpool.Pool
}

// NewAdjRepo konstruktor.
func NewAdjRepo(pool *pgxpool.Pool) *AdjRepo {
	return &AdjRepo{pool: pool}
}

// Pool exposes pool untuk koordinasi transaction multi-table di service layer.
func (r *AdjRepo) Pool() *pgxpool.Pool { return r.pool }

const adjColumns = `id, gudang_id, produk_id, satuan_id, qty, qty_konversi,
	kategori, alasan, catatan, user_id, created_at`

// Create insert satu baris adjustment dlm tx yang disediakan pemanggil.
// Tx di-pass agar bisa di-share dengan UPDATE stok dalam 1 transaction.
func (r *AdjRepo) Create(ctx context.Context, tx pgx.Tx, a *domain.StokAdjustment) error {
	const sql = `
		INSERT INTO stok_adjustment
		    (gudang_id, produk_id, satuan_id, qty, qty_konversi,
		     kategori, alasan, catatan, user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at`
	row := tx.QueryRow(ctx, sql,
		a.GudangID, a.ProdukID, a.SatuanID, a.Qty, a.QtyKonversi,
		a.Kategori, a.Alasan, a.Catatan, a.UserID,
	)
	if err := row.Scan(&a.ID, &a.CreatedAt); err != nil {
		return fmt.Errorf("insert stok_adjustment: %w", err)
	}
	return nil
}

// Get ambil satu adjustment by id (read langsung lewat pool, tanpa tx).
func (r *AdjRepo) Get(ctx context.Context, id int64) (*domain.StokAdjustment, error) {
	const sql = `
		SELECT a.id, a.gudang_id, a.produk_id, a.satuan_id, a.qty, a.qty_konversi,
			a.kategori, a.alasan, a.catatan, a.user_id, a.created_at,
			COALESCE(g.nama,''), COALESCE(p.nama,''),
			COALESCE(s.kode,''), COALESCE(u.nama_lengkap,'')
		FROM stok_adjustment a
		LEFT JOIN gudang g ON g.id = a.gudang_id
		LEFT JOIN produk p ON p.id = a.produk_id
		LEFT JOIN satuan s ON s.id = a.satuan_id
		LEFT JOIN "user" u ON u.id = a.user_id
		WHERE a.id = $1`
	var a domain.StokAdjustment
	var catatan *string
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&a.ID, &a.GudangID, &a.ProdukID, &a.SatuanID, &a.Qty, &a.QtyKonversi,
		&a.Kategori, &a.Alasan, &catatan, &a.UserID, &a.CreatedAt,
		&a.GudangNama, &a.ProdukNama, &a.SatuanKode, &a.UserNama,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAdjTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get stok_adjustment: %w", err)
	}
	a.Catatan = catatan
	return &a, nil
}

// List paginated dgn filter gudang/produk/kategori/range tanggal.
func (r *AdjRepo) List(ctx context.Context, f ListAdjFilter) ([]domain.StokAdjustment, int, error) {
	f.Normalize()

	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if f.GudangID != nil {
		conds = append(conds, fmt.Sprintf("a.gudang_id = $%d", idx))
		args = append(args, *f.GudangID)
		idx++
	}
	if f.ProdukID != nil {
		conds = append(conds, fmt.Sprintf("a.produk_id = $%d", idx))
		args = append(args, *f.ProdukID)
		idx++
	}
	if k := strings.TrimSpace(f.Kategori); k != "" {
		conds = append(conds, fmt.Sprintf("a.kategori = $%d", idx))
		args = append(args, k)
		idx++
	}
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("a.created_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("a.created_at < $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM stok_adjustment a WHERE "+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count stok_adjustment: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT a.id, a.gudang_id, a.produk_id, a.satuan_id, a.qty, a.qty_konversi,
			a.kategori, a.alasan, a.catatan, a.user_id, a.created_at,
			COALESCE(g.nama,''), COALESCE(p.nama,''),
			COALESCE(s.kode,''), COALESCE(u.nama_lengkap,'')
		FROM stok_adjustment a
		LEFT JOIN gudang g ON g.id = a.gudang_id
		LEFT JOIN produk p ON p.id = a.produk_id
		LEFT JOIN satuan s ON s.id = a.satuan_id
		LEFT JOIN "user" u ON u.id = a.user_id
		WHERE %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list stok_adjustment: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StokAdjustment, 0, f.PerPage)
	for rows.Next() {
		var a domain.StokAdjustment
		var catatan *string
		if err := rows.Scan(
			&a.ID, &a.GudangID, &a.ProdukID, &a.SatuanID, &a.Qty, &a.QtyKonversi,
			&a.Kategori, &a.Alasan, &catatan, &a.UserID, &a.CreatedAt,
			&a.GudangNama, &a.ProdukNama, &a.SatuanKode, &a.UserNama,
		); err != nil {
			return nil, 0, fmt.Errorf("scan stok_adjustment: %w", err)
		}
		a.Catatan = catatan
		out = append(out, a)
	}
	return out, total, rows.Err()
}

package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListPembayaranSupplierFilter filter list pembayaran.
type ListPembayaranSupplierFilter struct {
	DariTanggal   *time.Time
	SampaiTanggal *time.Time
	Page          int
	PerPage       int
}

// PembayaranSupplierRepo akses tabel pembayaran_supplier.
type PembayaranSupplierRepo struct {
	pool *pgxpool.Pool
}

// NewPembayaranSupplierRepo konstruktor.
func NewPembayaranSupplierRepo(pool *pgxpool.Pool) *PembayaranSupplierRepo {
	return &PembayaranSupplierRepo{pool: pool}
}

// Create insert pembayaran ke supplier.
func (r *PembayaranSupplierRepo) Create(ctx context.Context, p *domain.PembayaranSupplier) error {
	const sql = `
		INSERT INTO pembayaran_supplier (pembelian_id, supplier_id, tanggal, jumlah,
			metode, referensi, user_id, catatan)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at`
	var ref *string
	if strings.TrimSpace(p.Referensi) != "" {
		v := p.Referensi
		ref = &v
	}
	var catatan *string
	if strings.TrimSpace(p.Catatan) != "" {
		v := p.Catatan
		catatan = &v
	}
	err := r.pool.QueryRow(ctx, sql,
		p.PembelianID, p.SupplierID, p.Tanggal, p.Jumlah,
		p.Metode, ref, p.UserID, catatan,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return fmt.Errorf("create pembayaran supplier: %w", err)
	}
	return nil
}

// ListBySupplier paginated list per supplier.
func (r *PembayaranSupplierRepo) ListBySupplier(ctx context.Context, supplierID int64, f ListPembayaranSupplierFilter) ([]domain.PembayaranSupplier, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	conds := []string{"supplier_id = $1"}
	args := []any{supplierID}
	idx := 2
	if f.DariTanggal != nil {
		conds = append(conds, fmt.Sprintf("tanggal >= $%d", idx))
		args = append(args, *f.DariTanggal)
		idx++
	}
	if f.SampaiTanggal != nil {
		conds = append(conds, fmt.Sprintf("tanggal <= $%d", idx))
		args = append(args, *f.SampaiTanggal)
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM pembayaran_supplier WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pembayaran supplier: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT id, pembelian_id, supplier_id, tanggal, jumlah, metode,
			COALESCE(referensi,''), user_id, COALESCE(catatan,''), created_at
		FROM pembayaran_supplier
		WHERE %s
		ORDER BY tanggal DESC, id DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pembayaran supplier: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PembayaranSupplier, 0, f.PerPage)
	for rows.Next() {
		var p domain.PembayaranSupplier
		if err := rows.Scan(&p.ID, &p.PembelianID, &p.SupplierID, &p.Tanggal,
			&p.Jumlah, &p.Metode, &p.Referensi, &p.UserID, &p.Catatan, &p.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan pembayaran supplier: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// ListByPembelian semua pembayaran untuk satu pembelian (history).
func (r *PembayaranSupplierRepo) ListByPembelian(ctx context.Context, pembelianID int64) ([]domain.PembayaranSupplier, error) {
	const sql = `
		SELECT id, pembelian_id, supplier_id, tanggal, jumlah, metode,
			COALESCE(referensi,''), user_id, COALESCE(catatan,''), created_at
		FROM pembayaran_supplier
		WHERE pembelian_id = $1
		ORDER BY tanggal ASC, id ASC`
	rows, err := r.pool.Query(ctx, sql, pembelianID)
	if err != nil {
		return nil, fmt.Errorf("list pembayaran by pembelian: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PembayaranSupplier, 0, 4)
	for rows.Next() {
		var p domain.PembayaranSupplier
		if err := rows.Scan(&p.ID, &p.PembelianID, &p.SupplierID, &p.Tanggal,
			&p.Jumlah, &p.Metode, &p.Referensi, &p.UserID, &p.Catatan, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pembayaran: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SumByPembelian total pembayaran untuk satu pembelian (cents).
func (r *PembayaranSupplierRepo) SumByPembelian(ctx context.Context, pembelianID int64) (int64, error) {
	const sql = `
		SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran_supplier
		WHERE pembelian_id = $1`
	var v int64
	if err := r.pool.QueryRow(ctx, sql, pembelianID).Scan(&v); err != nil {
		return 0, fmt.Errorf("sum pembayaran: %w", err)
	}
	return v, nil
}

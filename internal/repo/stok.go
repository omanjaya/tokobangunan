package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StokDetail - row stok dengan field produk untuk display di UI.
type StokDetail struct {
	GudangID    int64
	GudangNama  string
	ProdukID    int64
	ProdukSKU   string
	ProdukNama  string
	Kategori    *string
	SatuanKode  string
	StokMinimum float64
	Qty         float64
	UpdatedAt   string
}

// ListStokFilter - filter listing stok per gudang.
type ListStokFilter struct {
	Query        string
	Kategori     *string
	LowStockOnly bool
	Page         int
	PerPage      int
}

// Normalize set default Page=1, PerPage=25 (max 100).
func (f *ListStokFilter) Normalize() {
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

// StokRepo akses tabel stok.
type StokRepo struct {
	pool *pgxpool.Pool
}

func NewStokRepo(pool *pgxpool.Pool) *StokRepo {
	return &StokRepo{pool: pool}
}

// Get - return qty stok pada gudang+produk. Return 0 (tanpa error) bila baris belum ada.
func (r *StokRepo) Get(ctx context.Context, gudangID, produkID int64) (float64, error) {
	const sql = `SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2`
	var qty float64
	if err := r.pool.QueryRow(ctx, sql, gudangID, produkID).Scan(&qty); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get stok: %w", err)
	}
	return qty, nil
}

// ListByGudang - listing stok di satu gudang dengan filter & pagination.
// LEFT JOIN produk agar produk yang stoknya 0 tetap muncul.
func (r *StokRepo) ListByGudang(ctx context.Context, gudangID int64, f ListStokFilter) ([]StokDetail, int, error) {
	f.Normalize()

	where := []string{"p.deleted_at IS NULL"}
	args := []any{gudangID}
	idx := 2

	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, fmt.Sprintf("(p.nama ILIKE $%d OR p.sku ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if f.Kategori != nil && strings.TrimSpace(*f.Kategori) != "" {
		where = append(where, fmt.Sprintf("p.kategori = $%d", idx))
		args = append(args, *f.Kategori)
		idx++
	}
	if f.LowStockOnly {
		where = append(where, "COALESCE(s.qty, 0) <= p.stok_minimum")
	}

	whereSQL := strings.Join(where, " AND ")

	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM produk p
		LEFT JOIN stok s ON s.produk_id = p.id AND s.gudang_id = $1
		WHERE %s`, whereSQL)
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count stok: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT p.id, p.sku, p.nama, p.kategori, p.stok_minimum,
		       sk.kode AS satuan_kode,
		       COALESCE(s.qty, 0) AS qty
		FROM produk p
		LEFT JOIN stok s ON s.produk_id = p.id AND s.gudang_id = $1
		LEFT JOIN satuan sk ON sk.id = p.satuan_kecil_id
		WHERE %s
		ORDER BY p.nama ASC
		LIMIT $%d OFFSET $%d`, whereSQL, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query stok: %w", err)
	}
	defer rows.Close()

	out := make([]StokDetail, 0, f.PerPage)
	for rows.Next() {
		var d StokDetail
		var satuanKode *string
		d.GudangID = gudangID
		if err := rows.Scan(&d.ProdukID, &d.ProdukSKU, &d.ProdukNama, &d.Kategori,
			&d.StokMinimum, &satuanKode, &d.Qty); err != nil {
			return nil, 0, fmt.Errorf("scan stok: %w", err)
		}
		if satuanKode != nil {
			d.SatuanKode = *satuanKode
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// ListLowStock - produk yang qty <= stok_minimum (semua gudang atau filter).
func (r *StokRepo) ListLowStock(ctx context.Context, gudangID *int64) ([]StokDetail, error) {
	args := []any{}
	gudangFilter := ""
	if gudangID != nil {
		gudangFilter = "AND s.gudang_id = $1"
		args = append(args, *gudangID)
	}
	sql := fmt.Sprintf(`
		SELECT s.gudang_id, g.nama, p.id, p.sku, p.nama, p.kategori,
		       p.stok_minimum, sk.kode, s.qty
		FROM stok s
		JOIN produk p ON p.id = s.produk_id AND p.deleted_at IS NULL
		JOIN gudang g ON g.id = s.gudang_id
		LEFT JOIN satuan sk ON sk.id = p.satuan_kecil_id
		WHERE s.qty <= p.stok_minimum %s
		ORDER BY g.nama ASC, p.nama ASC
		LIMIT 200`, gudangFilter)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query low stock: %w", err)
	}
	defer rows.Close()
	out := make([]StokDetail, 0, 32)
	for rows.Next() {
		var d StokDetail
		var satuanKode *string
		if err := rows.Scan(&d.GudangID, &d.GudangNama, &d.ProdukID, &d.ProdukSKU,
			&d.ProdukNama, &d.Kategori, &d.StokMinimum, &satuanKode, &d.Qty); err != nil {
			return nil, fmt.Errorf("scan low stock: %w", err)
		}
		if satuanKode != nil {
			d.SatuanKode = *satuanKode
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListAllByProduk - posisi stok 1 produk di semua gudang aktif.
func (r *StokRepo) ListAllByProduk(ctx context.Context, produkID int64) ([]StokDetail, error) {
	const sql = `
		SELECT g.id, g.nama, $1::bigint, p.sku, p.nama, p.kategori,
		       p.stok_minimum, sk.kode, COALESCE(s.qty, 0)
		FROM gudang g
		CROSS JOIN produk p
		LEFT JOIN stok s ON s.gudang_id = g.id AND s.produk_id = p.id
		LEFT JOIN satuan sk ON sk.id = p.satuan_kecil_id
		WHERE p.id = $1 AND p.deleted_at IS NULL AND g.is_active = TRUE
		ORDER BY g.nama ASC`
	rows, err := r.pool.Query(ctx, sql, produkID)
	if err != nil {
		return nil, fmt.Errorf("query stok by produk: %w", err)
	}
	defer rows.Close()
	out := make([]StokDetail, 0, 8)
	for rows.Next() {
		var d StokDetail
		var satuanKode *string
		if err := rows.Scan(&d.GudangID, &d.GudangNama, &d.ProdukID, &d.ProdukSKU,
			&d.ProdukNama, &d.Kategori, &d.StokMinimum, &satuanKode, &d.Qty); err != nil {
			return nil, fmt.Errorf("scan stok by produk: %w", err)
		}
		if satuanKode != nil {
			d.SatuanKode = *satuanKode
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MultiGudangSummary - peta produk_id -> gudang_id -> qty.
func (r *StokRepo) MultiGudangSummary(ctx context.Context, produkIDs []int64) (map[int64]map[int64]float64, error) {
	out := map[int64]map[int64]float64{}
	if len(produkIDs) == 0 {
		return out, nil
	}
	const sql = `SELECT produk_id, gudang_id, qty FROM stok WHERE produk_id = ANY($1)`
	rows, err := r.pool.Query(ctx, sql, produkIDs)
	if err != nil {
		return nil, fmt.Errorf("multi gudang summary: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid, gid int64
		var qty float64
		if err := rows.Scan(&pid, &gid, &qty); err != nil {
			return nil, err
		}
		if out[pid] == nil {
			out[pid] = map[int64]float64{}
		}
		out[pid][gid] = qty
	}
	return out, rows.Err()
}

// Package repo berisi data access layer (raw pgx) untuk modul master data.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListProdukFilter - parameter listing produk (search + paging).
type ListProdukFilter struct {
	Query    string
	Kategori *string
	IsActive *bool
	Page     int
	PerPage  int
}

// Normalize set default Page=1, PerPage=25 (max 100).
func (f *ListProdukFilter) Normalize() {
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

// ProdukRepo akses tabel produk.
type ProdukRepo struct {
	pool *pgxpool.Pool
}

func NewProdukRepo(pool *pgxpool.Pool) *ProdukRepo {
	return &ProdukRepo{pool: pool}
}

const produkColumns = `id, sku, nama, kategori, satuan_kecil_id, satuan_besar_id,
	faktor_konversi, stok_minimum, foto_url, is_active, lead_time_days, safety_stock,
	version, deleted_at, created_at, updated_at`

func scanProduk(row pgx.Row, p *domain.Produk) error {
	return row.Scan(&p.ID, &p.SKU, &p.Nama, &p.Kategori, &p.SatuanKecilID, &p.SatuanBesarID,
		&p.FaktorKonversi, &p.StokMinimum, &p.FotoURL, &p.IsActive,
		&p.LeadTimeDays, &p.SafetyStock,
		&p.Version, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt)
}

// UpdateFotoURL set/clear foto_url tanpa mempengaruhi version (kolom non-bisnis).
func (r *ProdukRepo) UpdateFotoURL(ctx context.Context, id int64, url *string) error {
	const sql = `UPDATE produk SET foto_url = $2 WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, sql, id, url)
	if err != nil {
		return fmt.Errorf("update foto_url: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProdukNotFound
	}
	return nil
}

// List - return rows + total count untuk pagination.
func (r *ProdukRepo) List(ctx context.Context, f ListProdukFilter) ([]domain.Produk, int, error) {
	f.Normalize()

	where := []string{"deleted_at IS NULL"}
	args := []any{}
	idx := 1

	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, fmt.Sprintf("(nama ILIKE $%d OR sku ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if f.Kategori != nil && strings.TrimSpace(*f.Kategori) != "" {
		where = append(where, fmt.Sprintf("kategori = $%d", idx))
		args = append(args, *f.Kategori)
		idx++
	}
	if f.IsActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *f.IsActive)
		idx++
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM produk WHERE " + whereSQL
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count produk: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM produk WHERE %s ORDER BY nama ASC LIMIT $%d OFFSET $%d`,
		produkColumns, whereSQL, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query produk: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Produk, 0, f.PerPage)
	for rows.Next() {
		var p domain.Produk
		if err := scanProduk(rows, &p); err != nil {
			return nil, 0, fmt.Errorf("scan produk: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter produk: %w", err)
	}
	return out, total, nil
}

// Search via trigram similarity (untuk autocomplete picker).
func (r *ProdukRepo) Search(ctx context.Context, q string, limit int) ([]domain.Produk, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	const sql = `
		SELECT ` + produkColumns + `
		FROM produk
		WHERE deleted_at IS NULL AND is_active = TRUE
		  AND (nama % $1 OR sku ILIKE $2)
		ORDER BY similarity(nama, $1) DESC, nama ASC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, sql, q, "%"+q+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search produk: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Produk, 0, limit)
	for rows.Next() {
		var p domain.Produk
		if err := scanProduk(rows, &p); err != nil {
			return nil, fmt.Errorf("scan produk: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByID ambil satu produk (deleted juga di-skip).
func (r *ProdukRepo) GetByID(ctx context.Context, id int64) (*domain.Produk, error) {
	const sql = `SELECT ` + produkColumns + ` FROM produk WHERE id = $1 AND deleted_at IS NULL`
	row := r.pool.QueryRow(ctx, sql, id)
	var p domain.Produk
	if err := scanProduk(row, &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProdukNotFound
		}
		return nil, fmt.Errorf("get produk: %w", err)
	}
	return &p, nil
}

// GetBySKU - cek dupe / lookup by SKU.
func (r *ProdukRepo) GetBySKU(ctx context.Context, sku string) (*domain.Produk, error) {
	const sql = `SELECT ` + produkColumns + ` FROM produk WHERE sku = $1 AND deleted_at IS NULL`
	row := r.pool.QueryRow(ctx, sql, sku)
	var p domain.Produk
	if err := scanProduk(row, &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProdukNotFound
		}
		return nil, fmt.Errorf("get produk by sku: %w", err)
	}
	return &p, nil
}

// Create insert + isi ID dari RETURNING.
func (r *ProdukRepo) Create(ctx context.Context, p *domain.Produk) error {
	const sql = `
		INSERT INTO produk (sku, nama, kategori, satuan_kecil_id, satuan_besar_id,
			faktor_konversi, stok_minimum, is_active, lead_time_days, safety_stock)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql,
		p.SKU, p.Nama, p.Kategori, p.SatuanKecilID, p.SatuanBesarID,
		p.FaktorKonversi, p.StokMinimum, p.IsActive, p.LeadTimeDays, p.SafetyStock)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("create produk: %w", err)
	}
	return nil
}

// Update tulis ulang seluruh field (kecuali deleted_at).
// Bila p.Version > 0, terapkan optimistic concurrency: row tidak akan ter-update
// jika version DB berbeda. Mismatch → domain.ErrConflict.
func (r *ProdukRepo) Update(ctx context.Context, p *domain.Produk) error {
	if p.Version > 0 {
		const sql = `
			UPDATE produk SET
				sku = $2, nama = $3, kategori = $4, satuan_kecil_id = $5, satuan_besar_id = $6,
				faktor_konversi = $7, stok_minimum = $8, is_active = $9,
				lead_time_days = $11, safety_stock = $12
			WHERE id = $1 AND deleted_at IS NULL AND version = $10
			RETURNING updated_at, version`
		row := r.pool.QueryRow(ctx, sql,
			p.ID, p.SKU, p.Nama, p.Kategori, p.SatuanKecilID, p.SatuanBesarID,
			p.FaktorKonversi, p.StokMinimum, p.IsActive, p.Version,
			p.LeadTimeDays, p.SafetyStock)
		if err := row.Scan(&p.UpdatedAt, &p.Version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Bedakan: row hilang vs version mismatch. Cek eksistensi.
				var exists bool
				if e := r.pool.QueryRow(ctx,
					`SELECT EXISTS(SELECT 1 FROM produk WHERE id = $1 AND deleted_at IS NULL)`,
					p.ID).Scan(&exists); e == nil && exists {
					return domain.ErrConflict
				}
				return domain.ErrProdukNotFound
			}
			return fmt.Errorf("update produk: %w", err)
		}
		return nil
	}
	const sql = `
		UPDATE produk SET
			sku = $2, nama = $3, kategori = $4, satuan_kecil_id = $5, satuan_besar_id = $6,
			faktor_konversi = $7, stok_minimum = $8, is_active = $9,
			lead_time_days = $10, safety_stock = $11
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at, version`
	row := r.pool.QueryRow(ctx, sql,
		p.ID, p.SKU, p.Nama, p.Kategori, p.SatuanKecilID, p.SatuanBesarID,
		p.FaktorKonversi, p.StokMinimum, p.IsActive,
		p.LeadTimeDays, p.SafetyStock)
	if err := row.Scan(&p.UpdatedAt, &p.Version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrProdukNotFound
		}
		return fmt.Errorf("update produk: %w", err)
	}
	return nil
}

// SoftDelete - set deleted_at = now().
func (r *ProdukRepo) SoftDelete(ctx context.Context, id int64) error {
	const sql = `UPDATE produk SET deleted_at = now(), is_active = FALSE
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("soft delete produk: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProdukNotFound
	}
	return nil
}

// ListKategori - distinct kategori (untuk filter dropdown).
func (r *ProdukRepo) ListKategori(ctx context.Context) ([]string, error) {
	const sql = `SELECT DISTINCT kategori FROM produk
		WHERE deleted_at IS NULL AND kategori IS NOT NULL AND kategori <> ''
		ORDER BY kategori`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list kategori: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

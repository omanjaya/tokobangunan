package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListSupplierFilter filter & pagination input untuk SupplierRepo.List.
type ListSupplierFilter struct {
	Query    string
	IsActive *bool
	Page     int
	PerPage  int
}

// SupplierRepo CRUD raw pgx untuk tabel supplier.
type SupplierRepo struct {
	pool *pgxpool.Pool
}

// NewSupplierRepo konstruktor.
func NewSupplierRepo(pool *pgxpool.Pool) *SupplierRepo {
	return &SupplierRepo{pool: pool}
}

const supplierColumns = `id, kode, nama, alamat, kontak, catatan, is_active,
	deleted_at, created_at, updated_at`

func scanSupplier(row pgx.Row, s *domain.Supplier) error {
	return row.Scan(&s.ID, &s.Kode, &s.Nama, &s.Alamat, &s.Kontak, &s.Catatan,
		&s.IsActive, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt)
}

// List paginasi + filter. Mengembalikan items, total, error.
func (r *SupplierRepo) List(ctx context.Context, f ListSupplierFilter) ([]domain.Supplier, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}

	conds := []string{"deleted_at IS NULL"}
	args := []any{}
	idx := 1

	if q := strings.TrimSpace(f.Query); q != "" {
		conds = append(conds, fmt.Sprintf("(nama ILIKE $%d OR kode ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if f.IsActive != nil {
		conds = append(conds, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *f.IsActive)
		idx++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	countSQL := "SELECT COUNT(*) FROM supplier " + where
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supplier: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`SELECT %s FROM supplier %s ORDER BY nama ASC LIMIT $%d OFFSET $%d`,
		supplierColumns, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list supplier: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Supplier, 0, f.PerPage)
	for rows.Next() {
		var s domain.Supplier
		if err := scanSupplier(rows, &s); err != nil {
			return nil, 0, fmt.Errorf("scan supplier: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter supplier: %w", err)
	}
	return items, total, nil
}

// GetByID load satu supplier by id (tidak termasuk soft-deleted).
func (r *SupplierRepo) GetByID(ctx context.Context, id int64) (*domain.Supplier, error) {
	sql := fmt.Sprintf(`SELECT %s FROM supplier WHERE id = $1 AND deleted_at IS NULL`, supplierColumns)
	var s domain.Supplier
	err := scanSupplier(r.pool.QueryRow(ctx, sql, id), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSupplierTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get supplier: %w", err)
	}
	return &s, nil
}

// GetByKode lookup unik untuk validasi duplikat.
func (r *SupplierRepo) GetByKode(ctx context.Context, kode string) (*domain.Supplier, error) {
	sql := fmt.Sprintf(`SELECT %s FROM supplier WHERE kode = $1 AND deleted_at IS NULL`, supplierColumns)
	var s domain.Supplier
	err := scanSupplier(r.pool.QueryRow(ctx, sql, kode), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSupplierTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get supplier by kode: %w", err)
	}
	return &s, nil
}

// Create insert baru, set ID & timestamps.
func (r *SupplierRepo) Create(ctx context.Context, s *domain.Supplier) error {
	const sql = `INSERT INTO supplier (kode, nama, alamat, kontak, catatan, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, sql,
		s.Kode, s.Nama, s.Alamat, s.Kontak, s.Catatan, s.IsActive,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrSupplierKodeDuplicate
		}
		return fmt.Errorf("create supplier: %w", err)
	}
	return nil
}

// Update field selain id.
func (r *SupplierRepo) Update(ctx context.Context, s *domain.Supplier) error {
	const sql = `UPDATE supplier SET
		kode=$1, nama=$2, alamat=$3, kontak=$4, catatan=$5,
		is_active=$6, updated_at=now()
		WHERE id=$7 AND deleted_at IS NULL
		RETURNING updated_at`
	err := r.pool.QueryRow(ctx, sql,
		s.Kode, s.Nama, s.Alamat, s.Kontak, s.Catatan, s.IsActive, s.ID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrSupplierTidakDitemukan
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrSupplierKodeDuplicate
		}
		return fmt.Errorf("update supplier: %w", err)
	}
	return nil
}

// SoftDelete tandai deleted_at = now().
func (r *SupplierRepo) SoftDelete(ctx context.Context, id int64) error {
	const sql = `UPDATE supplier SET deleted_at = now(), is_active = FALSE
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("soft delete supplier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSupplierTidakDitemukan
	}
	return nil
}

// Package repo berisi data access layer (raw pgx). Tidak ada business logic.
// Convention: List(filter), GetByID, GetByKode, Create, Update, SoftDelete.
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

// ListMitraFilter filter & pagination input untuk MitraRepo.List.
type ListMitraFilter struct {
	Query    string
	Tipe     *string
	IsActive *bool
	Page     int
	PerPage  int
}

// MitraRepo CRUD raw pgx untuk tabel mitra.
type MitraRepo struct {
	pool *pgxpool.Pool
}

// NewMitraRepo konstruktor.
func NewMitraRepo(pool *pgxpool.Pool) *MitraRepo {
	return &MitraRepo{pool: pool}
}

const mitraColumns = `id, kode, nama, alamat, kontak, npwp, tipe, limit_kredit,
	jatuh_tempo_hari, gudang_default_id, catatan, is_active, version, deleted_at,
	created_at, updated_at`

func scanMitra(row pgx.Row, m *domain.Mitra) error {
	return row.Scan(&m.ID, &m.Kode, &m.Nama, &m.Alamat, &m.Kontak, &m.NPWP,
		&m.Tipe, &m.LimitKredit, &m.JatuhTempoHari, &m.GudangDefaultID,
		&m.Catatan, &m.IsActive, &m.Version, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt)
}

// List paginasi + filter. Mengembalikan items, total, error.
func (r *MitraRepo) List(ctx context.Context, f ListMitraFilter) ([]domain.Mitra, int, error) {
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
	if f.Tipe != nil && *f.Tipe != "" {
		conds = append(conds, fmt.Sprintf("tipe = $%d", idx))
		args = append(args, *f.Tipe)
		idx++
	}
	if f.IsActive != nil {
		conds = append(conds, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *f.IsActive)
		idx++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	countSQL := "SELECT COUNT(*) FROM mitra " + where
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count mitra: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`SELECT %s FROM mitra %s ORDER BY nama ASC LIMIT $%d OFFSET $%d`,
		mitraColumns, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list mitra: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Mitra, 0, f.PerPage)
	for rows.Next() {
		var m domain.Mitra
		if err := scanMitra(rows, &m); err != nil {
			return nil, 0, fmt.Errorf("scan mitra: %w", err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter mitra: %w", err)
	}
	return items, total, nil
}

// Search trigram autocomplete pakai index nama_trgm. Limit default 10.
func (r *MitraRepo) Search(ctx context.Context, query string, limit int) ([]domain.Mitra, error) {
	if limit <= 0 {
		limit = 10
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return []domain.Mitra{}, nil
	}
	sql := fmt.Sprintf(`SELECT %s FROM mitra
		WHERE deleted_at IS NULL AND is_active = TRUE
		  AND (nama ILIKE $1 OR kode ILIKE $1)
		ORDER BY similarity(nama, $2) DESC, nama ASC
		LIMIT $3`, mitraColumns)
	rows, err := r.pool.Query(ctx, sql, "%"+q+"%", q, limit)
	if err != nil {
		return nil, fmt.Errorf("search mitra: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Mitra, 0, limit)
	for rows.Next() {
		var m domain.Mitra
		if err := scanMitra(rows, &m); err != nil {
			return nil, fmt.Errorf("scan mitra: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// GetByID load satu mitra by id (tidak termasuk soft-deleted).
func (r *MitraRepo) GetByID(ctx context.Context, id int64) (*domain.Mitra, error) {
	sql := fmt.Sprintf(`SELECT %s FROM mitra WHERE id = $1 AND deleted_at IS NULL`, mitraColumns)
	var m domain.Mitra
	err := scanMitra(r.pool.QueryRow(ctx, sql, id), &m)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMitraTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get mitra: %w", err)
	}
	return &m, nil
}

// GetByKode lookup unik untuk validasi duplikat.
func (r *MitraRepo) GetByKode(ctx context.Context, kode string) (*domain.Mitra, error) {
	sql := fmt.Sprintf(`SELECT %s FROM mitra WHERE kode = $1 AND deleted_at IS NULL`, mitraColumns)
	var m domain.Mitra
	err := scanMitra(r.pool.QueryRow(ctx, sql, kode), &m)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMitraTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get mitra by kode: %w", err)
	}
	return &m, nil
}

// Create insert baru, set ID & timestamps.
func (r *MitraRepo) Create(ctx context.Context, m *domain.Mitra) error {
	const sql = `INSERT INTO mitra (kode, nama, alamat, kontak, npwp, tipe,
		limit_kredit, jatuh_tempo_hari, gudang_default_id, catatan, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, sql,
		m.Kode, m.Nama, m.Alamat, m.Kontak, m.NPWP, m.Tipe,
		m.LimitKredit, m.JatuhTempoHari, m.GudangDefaultID, m.Catatan, m.IsActive,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrMitraKodeDuplicate
		}
		return fmt.Errorf("create mitra: %w", err)
	}
	return nil
}

// Update update field selain id, kode tetap boleh diubah jika unik.
// Bila m.Version > 0, terapkan optimistic concurrency.
func (r *MitraRepo) Update(ctx context.Context, m *domain.Mitra) error {
	if m.Version > 0 {
		const sql = `UPDATE mitra SET
			kode=$1, nama=$2, alamat=$3, kontak=$4, npwp=$5, tipe=$6,
			limit_kredit=$7, jatuh_tempo_hari=$8, gudang_default_id=$9,
			catatan=$10, is_active=$11, updated_at=now()
			WHERE id=$12 AND deleted_at IS NULL AND version=$13
			RETURNING updated_at, version`
		err := r.pool.QueryRow(ctx, sql,
			m.Kode, m.Nama, m.Alamat, m.Kontak, m.NPWP, m.Tipe,
			m.LimitKredit, m.JatuhTempoHari, m.GudangDefaultID, m.Catatan,
			m.IsActive, m.ID, m.Version,
		).Scan(&m.UpdatedAt, &m.Version)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if e := r.pool.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM mitra WHERE id = $1 AND deleted_at IS NULL)`,
				m.ID).Scan(&exists); e == nil && exists {
				return domain.ErrConflict
			}
			return domain.ErrMitraTidakDitemukan
		}
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domain.ErrMitraKodeDuplicate
			}
			return fmt.Errorf("update mitra: %w", err)
		}
		return nil
	}
	const sql = `UPDATE mitra SET
		kode=$1, nama=$2, alamat=$3, kontak=$4, npwp=$5, tipe=$6,
		limit_kredit=$7, jatuh_tempo_hari=$8, gudang_default_id=$9,
		catatan=$10, is_active=$11, updated_at=now()
		WHERE id=$12 AND deleted_at IS NULL
		RETURNING updated_at, version`
	err := r.pool.QueryRow(ctx, sql,
		m.Kode, m.Nama, m.Alamat, m.Kontak, m.NPWP, m.Tipe,
		m.LimitKredit, m.JatuhTempoHari, m.GudangDefaultID, m.Catatan,
		m.IsActive, m.ID,
	).Scan(&m.UpdatedAt, &m.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrMitraTidakDitemukan
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrMitraKodeDuplicate
		}
		return fmt.Errorf("update mitra: %w", err)
	}
	return nil
}

// SoftDelete tandai deleted_at = now().
func (r *MitraRepo) SoftDelete(ctx context.Context, id int64) error {
	const sql = `UPDATE mitra SET deleted_at = now(), is_active = FALSE
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("soft delete mitra: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMitraTidakDitemukan
	}
	return nil
}

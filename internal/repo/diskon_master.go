package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// DiskonMasterRepo akses tabel diskon_master.
type DiskonMasterRepo struct {
	pool *pgxpool.Pool
}

func NewDiskonMasterRepo(pool *pgxpool.Pool) *DiskonMasterRepo {
	return &DiskonMasterRepo{pool: pool}
}

const diskonColumns = `id, kode, nama, tipe, nilai, min_subtotal, max_diskon,
	berlaku_dari, berlaku_sampai, is_active, created_at, updated_at`

func scanDiskon(row pgx.Row, d *domain.DiskonMaster) error {
	return row.Scan(
		&d.ID, &d.Kode, &d.Nama, &d.Tipe, &d.Nilai,
		&d.MinSubtotal, &d.MaxDiskon,
		&d.BerlakuDari, &d.BerlakuSampai,
		&d.IsActive, &d.CreatedAt, &d.UpdatedAt,
	)
}

// DiskonFilter - filter list.
type DiskonFilter struct {
	OnlyActive bool
	AtTime     *time.Time // jika set, batasi yg berlaku pada saat ini
}

// List - daftar diskon. Default urut by kode.
func (r *DiskonMasterRepo) List(ctx context.Context, f DiskonFilter) ([]domain.DiskonMaster, error) {
	sql := `SELECT ` + diskonColumns + ` FROM diskon_master WHERE 1=1`
	args := []any{}
	if f.OnlyActive {
		sql += ` AND is_active = TRUE`
	}
	if f.AtTime != nil {
		args = append(args, *f.AtTime)
		sql += fmt.Sprintf(` AND berlaku_dari <= $%d AND (berlaku_sampai IS NULL OR berlaku_sampai >= $%d)`, len(args), len(args))
	}
	sql += ` ORDER BY kode ASC`

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list diskon: %w", err)
	}
	defer rows.Close()
	out := []domain.DiskonMaster{}
	for rows.Next() {
		var d domain.DiskonMaster
		if err := scanDiskon(rows, &d); err != nil {
			return nil, fmt.Errorf("scan diskon: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DiskonMasterRepo) GetByID(ctx context.Context, id int64) (*domain.DiskonMaster, error) {
	const sql = `SELECT ` + diskonColumns + ` FROM diskon_master WHERE id = $1`
	var d domain.DiskonMaster
	if err := scanDiskon(r.pool.QueryRow(ctx, sql, id), &d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDiskonNotFound
		}
		return nil, fmt.Errorf("get diskon: %w", err)
	}
	return &d, nil
}

func (r *DiskonMasterRepo) GetByKode(ctx context.Context, kode string) (*domain.DiskonMaster, error) {
	const sql = `SELECT ` + diskonColumns + ` FROM diskon_master WHERE kode = $1`
	var d domain.DiskonMaster
	if err := scanDiskon(r.pool.QueryRow(ctx, sql, kode), &d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDiskonNotFound
		}
		return nil, fmt.Errorf("get diskon by kode: %w", err)
	}
	return &d, nil
}

func (r *DiskonMasterRepo) Create(ctx context.Context, d *domain.DiskonMaster) error {
	const sql = `
		INSERT INTO diskon_master
			(kode, nama, tipe, nilai, min_subtotal, max_diskon, berlaku_dari, berlaku_sampai, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql,
		d.Kode, d.Nama, d.Tipe, d.Nilai,
		d.MinSubtotal, d.MaxDiskon,
		d.BerlakuDari, d.BerlakuSampai, d.IsActive,
	)
	if err := row.Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return fmt.Errorf("create diskon: %w", err)
	}
	return nil
}

func (r *DiskonMasterRepo) Update(ctx context.Context, d *domain.DiskonMaster) error {
	const sql = `
		UPDATE diskon_master SET
			kode = $2, nama = $3, tipe = $4, nilai = $5,
			min_subtotal = $6, max_diskon = $7,
			berlaku_dari = $8, berlaku_sampai = $9, is_active = $10,
			updated_at = now()
		WHERE id = $1
		RETURNING updated_at`
	row := r.pool.QueryRow(ctx, sql,
		d.ID, d.Kode, d.Nama, d.Tipe, d.Nilai,
		d.MinSubtotal, d.MaxDiskon,
		d.BerlakuDari, d.BerlakuSampai, d.IsActive,
	)
	if err := row.Scan(&d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDiskonNotFound
		}
		return fmt.Errorf("update diskon: %w", err)
	}
	return nil
}

// SetActive - toggle aktif (juga dipakai sbg soft-delete via active=false).
func (r *DiskonMasterRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const sql = `UPDATE diskon_master SET is_active = $2, updated_at = now() WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, active)
	if err != nil {
		return fmt.Errorf("set active diskon: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDiskonNotFound
	}
	return nil
}

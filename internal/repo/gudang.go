package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// GudangRepo akses tabel gudang.
type GudangRepo struct {
	pool *pgxpool.Pool
}

func NewGudangRepo(pool *pgxpool.Pool) *GudangRepo {
	return &GudangRepo{pool: pool}
}

const gudangColumns = `id, kode, nama, alamat, telepon, is_active, created_at, updated_at`

func scanGudang(row pgx.Row, g *domain.Gudang) error {
	return row.Scan(&g.ID, &g.Kode, &g.Nama, &g.Alamat, &g.Telepon,
		&g.IsActive, &g.CreatedAt, &g.UpdatedAt)
}

// List - semua gudang. Bila includeInactive=false, hanya yang is_active=true.
func (r *GudangRepo) List(ctx context.Context, includeInactive bool) ([]domain.Gudang, error) {
	sql := `SELECT ` + gudangColumns + ` FROM gudang`
	if !includeInactive {
		sql += ` WHERE is_active = TRUE`
	}
	sql += ` ORDER BY nama ASC`

	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query gudang: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Gudang, 0, 8)
	for rows.Next() {
		var g domain.Gudang
		if err := scanGudang(rows, &g); err != nil {
			return nil, fmt.Errorf("scan gudang: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *GudangRepo) GetByID(ctx context.Context, id int64) (*domain.Gudang, error) {
	const sql = `SELECT ` + gudangColumns + ` FROM gudang WHERE id = $1`
	row := r.pool.QueryRow(ctx, sql, id)
	var g domain.Gudang
	if err := scanGudang(row, &g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGudangNotFound
		}
		return nil, fmt.Errorf("get gudang: %w", err)
	}
	return &g, nil
}

func (r *GudangRepo) GetByKode(ctx context.Context, kode string) (*domain.Gudang, error) {
	const sql = `SELECT ` + gudangColumns + ` FROM gudang WHERE kode = $1`
	row := r.pool.QueryRow(ctx, sql, kode)
	var g domain.Gudang
	if err := scanGudang(row, &g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGudangNotFound
		}
		return nil, fmt.Errorf("get gudang by kode: %w", err)
	}
	return &g, nil
}

func (r *GudangRepo) Create(ctx context.Context, g *domain.Gudang) error {
	const sql = `
		INSERT INTO gudang (kode, nama, alamat, telepon, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql, g.Kode, g.Nama, g.Alamat, g.Telepon, g.IsActive)
	if err := row.Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return fmt.Errorf("create gudang: %w", err)
	}
	return nil
}

func (r *GudangRepo) Update(ctx context.Context, g *domain.Gudang) error {
	const sql = `
		UPDATE gudang SET
			kode = $2, nama = $3, alamat = $4, telepon = $5, is_active = $6
		WHERE id = $1
		RETURNING updated_at`
	row := r.pool.QueryRow(ctx, sql, g.ID, g.Kode, g.Nama, g.Alamat, g.Telepon, g.IsActive)
	if err := row.Scan(&g.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrGudangNotFound
		}
		return fmt.Errorf("update gudang: %w", err)
	}
	return nil
}

func (r *GudangRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const sql = `UPDATE gudang SET is_active = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, active)
	if err != nil {
		return fmt.Errorf("set active gudang: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGudangNotFound
	}
	return nil
}

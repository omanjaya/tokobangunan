package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// SatuanRepo akses tabel satuan (master data fixed, tanpa delete).
type SatuanRepo struct {
	pool *pgxpool.Pool
}

func NewSatuanRepo(pool *pgxpool.Pool) *SatuanRepo {
	return &SatuanRepo{pool: pool}
}

const satuanColumns = `id, kode, nama, created_at, updated_at`

func scanSatuan(row pgx.Row, s *domain.Satuan) error {
	return row.Scan(&s.ID, &s.Kode, &s.Nama, &s.CreatedAt, &s.UpdatedAt)
}

// List urutkan by kode.
func (r *SatuanRepo) List(ctx context.Context) ([]domain.Satuan, error) {
	const sql = `SELECT ` + satuanColumns + ` FROM satuan ORDER BY kode ASC`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list satuan: %w", err)
	}
	defer rows.Close()
	out := []domain.Satuan{}
	for rows.Next() {
		var s domain.Satuan
		if err := scanSatuan(rows, &s); err != nil {
			return nil, fmt.Errorf("scan satuan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SatuanRepo) GetByID(ctx context.Context, id int64) (*domain.Satuan, error) {
	const sql = `SELECT ` + satuanColumns + ` FROM satuan WHERE id = $1`
	var s domain.Satuan
	if err := scanSatuan(r.pool.QueryRow(ctx, sql, id), &s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSatuanNotFound
		}
		return nil, fmt.Errorf("get satuan: %w", err)
	}
	return &s, nil
}

func (r *SatuanRepo) GetByKode(ctx context.Context, kode string) (*domain.Satuan, error) {
	const sql = `SELECT ` + satuanColumns + ` FROM satuan WHERE kode = $1`
	var s domain.Satuan
	if err := scanSatuan(r.pool.QueryRow(ctx, sql, kode), &s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSatuanNotFound
		}
		return nil, fmt.Errorf("get satuan by kode: %w", err)
	}
	return &s, nil
}

func (r *SatuanRepo) Create(ctx context.Context, s *domain.Satuan) error {
	const sql = `INSERT INTO satuan (kode, nama)
		VALUES ($1, $2) RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql, s.Kode, s.Nama)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("create satuan: %w", err)
	}
	return nil
}

package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// PrinterTemplateRepo akses tabel printer_template.
type PrinterTemplateRepo struct {
	pool *pgxpool.Pool
}

func NewPrinterTemplateRepo(pool *pgxpool.Pool) *PrinterTemplateRepo {
	return &PrinterTemplateRepo{pool: pool}
}

const printerTemplateColumns = `id, gudang_id, jenis, nama, lebar_char, panjang_baris,
	offset_x, offset_y, koordinat::text, preview, is_default, created_at, updated_at`

func scanPrinterTemplate(row pgx.Row, t *domain.PrinterTemplate) error {
	return row.Scan(&t.ID, &t.GudangID, &t.Jenis, &t.Nama, &t.LebarChar, &t.PanjangBaris,
		&t.OffsetX, &t.OffsetY, &t.Koordinat, &t.Preview, &t.IsDefault,
		&t.CreatedAt, &t.UpdatedAt)
}

// List semua template, urut by gudang_id then jenis then nama.
func (r *PrinterTemplateRepo) List(ctx context.Context) ([]domain.PrinterTemplate, error) {
	const sql = `SELECT ` + printerTemplateColumns + `
		FROM printer_template
		ORDER BY gudang_id ASC, jenis ASC, nama ASC`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query printer_template: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PrinterTemplate, 0, 16)
	for rows.Next() {
		var t domain.PrinterTemplate
		if err := scanPrinterTemplate(rows, &t); err != nil {
			return nil, fmt.Errorf("scan printer_template: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PrinterTemplateRepo) GetByID(ctx context.Context, id int64) (*domain.PrinterTemplate, error) {
	const sql = `SELECT ` + printerTemplateColumns + ` FROM printer_template WHERE id = $1`
	row := r.pool.QueryRow(ctx, sql, id)
	var t domain.PrinterTemplate
	if err := scanPrinterTemplate(row, &t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPrinterTemplateNotFound
		}
		return nil, fmt.Errorf("get printer_template: %w", err)
	}
	return &t, nil
}

func (r *PrinterTemplateRepo) Create(ctx context.Context, t *domain.PrinterTemplate) error {
	const sql = `
		INSERT INTO printer_template (gudang_id, jenis, nama, lebar_char, panjang_baris,
			offset_x, offset_y, koordinat, preview, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10)
		RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql,
		t.GudangID, t.Jenis, t.Nama, t.LebarChar, t.PanjangBaris,
		t.OffsetX, t.OffsetY, t.Koordinat, t.Preview, t.IsDefault)
	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return fmt.Errorf("create printer_template: %w", err)
	}
	return nil
}

func (r *PrinterTemplateRepo) Update(ctx context.Context, t *domain.PrinterTemplate) error {
	const sql = `
		UPDATE printer_template SET
			gudang_id = $2, jenis = $3, nama = $4,
			lebar_char = $5, panjang_baris = $6,
			offset_x = $7, offset_y = $8,
			koordinat = $9::jsonb, preview = $10, is_default = $11
		WHERE id = $1
		RETURNING updated_at`
	row := r.pool.QueryRow(ctx, sql,
		t.ID, t.GudangID, t.Jenis, t.Nama, t.LebarChar, t.PanjangBaris,
		t.OffsetX, t.OffsetY, t.Koordinat, t.Preview, t.IsDefault)
	if err := row.Scan(&t.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPrinterTemplateNotFound
		}
		return fmt.Errorf("update printer_template: %w", err)
	}
	return nil
}

func (r *PrinterTemplateRepo) Delete(ctx context.Context, id int64) error {
	const sql = `DELETE FROM printer_template WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("delete printer_template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPrinterTemplateNotFound
	}
	return nil
}

// SetDefault menetapkan template id sebagai default untuk (gudang_id, jenis).
// Template lain dengan kombinasi sama akan di-unset.
func (r *PrinterTemplateRepo) SetDefault(ctx context.Context, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var gudangID int64
	var jenis string
	if err := tx.QueryRow(ctx,
		`SELECT gudang_id, jenis FROM printer_template WHERE id = $1`, id,
	).Scan(&gudangID, &jenis); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrPrinterTemplateNotFound
		}
		return fmt.Errorf("lookup printer_template: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE printer_template SET is_default = FALSE
		 WHERE gudang_id = $1 AND jenis = $2 AND id <> $3`,
		gudangID, jenis, id,
	); err != nil {
		return fmt.Errorf("clear default: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE printer_template SET is_default = TRUE WHERE id = $1`, id,
	); err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	return tx.Commit(ctx)
}

// ExistsByName cek dupe (gudang_id, jenis, nama) excluding excludeID.
func (r *PrinterTemplateRepo) ExistsByName(ctx context.Context,
	gudangID int64, jenis, nama string, excludeID int64,
) (bool, error) {
	const sql = `SELECT 1 FROM printer_template
		WHERE gudang_id = $1 AND jenis = $2 AND nama = $3 AND id <> $4
		LIMIT 1`
	var x int
	err := r.pool.QueryRow(ctx, sql, gudangID, jenis, nama, excludeID).Scan(&x)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check dupe printer_template: %w", err)
	}
	return true, nil
}

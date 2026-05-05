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

// ListTabunganFilter filter list ledger tabungan.
type ListTabunganFilter struct {
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}

// Normalize default page/perpage.
func (f *ListTabunganFilter) Normalize() {
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

// TabunganMitraRepo akses tabel tabungan_mitra.
type TabunganMitraRepo struct {
	pool *pgxpool.Pool
}

// NewTabunganMitraRepo konstruktor.
func NewTabunganMitraRepo(pool *pgxpool.Pool) *TabunganMitraRepo {
	return &TabunganMitraRepo{pool: pool}
}

const tabunganColumns = `id, mitra_id, tanggal, debit, kredit, saldo,
	COALESCE(catatan, ''), user_id, created_at`

func scanTabungan(row pgx.Row, t *domain.TabunganMitra) error {
	return row.Scan(&t.ID, &t.MitraID, &t.Tanggal, &t.Debit, &t.Kredit,
		&t.Saldo, &t.Catatan, &t.UserID, &t.CreatedAt)
}

// Insert tambah baris ledger. Saldo dihitung dari saldo terakhir + debit - kredit.
// Pakai SELECT FOR UPDATE pada baris terakhir mitra agar concurrency aman.
// Bila kredit > saldo terakhir → return ErrTabunganSaldoKurang.
func (r *TabunganMitraRepo) Insert(ctx context.Context, t *domain.TabunganMitra) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const lockSQL = `SELECT saldo FROM tabungan_mitra
		WHERE mitra_id = $1
		ORDER BY tanggal DESC, id DESC
		LIMIT 1
		FOR UPDATE`
	var saldoLama int64
	row := tx.QueryRow(ctx, lockSQL, t.MitraID)
	if err := row.Scan(&saldoLama); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock saldo: %w", err)
		}
		saldoLama = 0
	}

	saldoBaru := saldoLama + t.Debit - t.Kredit
	if saldoBaru < 0 {
		return domain.ErrTabunganSaldoKurang
	}
	t.Saldo = saldoBaru

	const insSQL = `INSERT INTO tabungan_mitra
		(mitra_id, tanggal, debit, kredit, saldo, catatan, user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at`
	var catatan *string
	if v := strings.TrimSpace(t.Catatan); v != "" {
		catatan = &v
	}
	if err := tx.QueryRow(ctx, insSQL,
		t.MitraID, t.Tanggal, t.Debit, t.Kredit, t.Saldo, catatan, t.UserID,
	).Scan(&t.ID, &t.CreatedAt); err != nil {
		return fmt.Errorf("insert tabungan: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx tabungan: %w", err)
	}
	return nil
}

// ListByMitra paginated history per mitra.
func (r *TabunganMitraRepo) ListByMitra(ctx context.Context, mitraID int64, f ListTabunganFilter) ([]domain.TabunganMitra, int, error) {
	f.Normalize()
	conds := []string{"mitra_id = $1"}
	args := []any{mitraID}
	idx := 2
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("tanggal >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("tanggal <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM tabungan_mitra WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tabungan: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`SELECT %s FROM tabungan_mitra WHERE %s
		ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		tabunganColumns, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tabungan: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TabunganMitra, 0, f.PerPage)
	for rows.Next() {
		var t domain.TabunganMitra
		if err := scanTabungan(rows, &t); err != nil {
			return nil, 0, fmt.Errorf("scan tabungan: %w", err)
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// GetSaldo saldo terkini mitra (saldo dari baris terakhir, atau 0).
func (r *TabunganMitraRepo) GetSaldo(ctx context.Context, mitraID int64) (int64, error) {
	const sql = `SELECT saldo FROM tabungan_mitra
		WHERE mitra_id = $1
		ORDER BY tanggal DESC, id DESC
		LIMIT 1`
	var v int64
	err := r.pool.QueryRow(ctx, sql, mitraID).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get saldo: %w", err)
	}
	return v, nil
}

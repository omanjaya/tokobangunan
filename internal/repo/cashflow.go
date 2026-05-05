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

// CashflowRepo akses tabel cashflow + cashflow_kategori.
type CashflowRepo struct {
	pool *pgxpool.Pool
}

func NewCashflowRepo(pool *pgxpool.Pool) *CashflowRepo {
	return &CashflowRepo{pool: pool}
}

// ListCashflowFilter parameter listing.
type ListCashflowFilter struct {
	From     *time.Time
	To       *time.Time
	GudangID *int64
	Tipe     string // "" / "masuk" / "keluar"
	Kategori string
	Page     int
	PerPage  int
}

// Normalize set default Page=1, PerPage=25.
func (f *ListCashflowFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	if f.PerPage > 200 {
		f.PerPage = 200
	}
}

const cashflowColumns = `id, nomor, tanggal, gudang_id, tipe, kategori, deskripsi,
	jumlah, metode, referensi, user_id, catatan, created_at`

func scanCashflow(row pgx.Row, c *domain.Cashflow) error {
	var deskripsi, referensi, catatan *string
	var tipe string
	if err := row.Scan(&c.ID, &c.Nomor, &c.Tanggal, &c.GudangID, &tipe, &c.Kategori,
		&deskripsi, &c.Jumlah, &c.Metode, &referensi, &c.UserID, &catatan, &c.CreatedAt); err != nil {
		return err
	}
	c.Tipe = domain.CashflowTipe(tipe)
	if deskripsi != nil {
		c.Deskripsi = *deskripsi
	}
	if referensi != nil {
		c.Referensi = *referensi
	}
	if catatan != nil {
		c.Catatan = *catatan
	}
	return nil
}

// Create insert cashflow.
func (r *CashflowRepo) Create(ctx context.Context, c *domain.Cashflow) error {
	const sql = `INSERT INTO cashflow
		(nomor, tanggal, gudang_id, tipe, kategori, deskripsi,
		 jumlah, metode, referensi, user_id, catatan)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at`
	var deskripsi, referensi, catatan *string
	if s := strings.TrimSpace(c.Deskripsi); s != "" {
		deskripsi = &s
	}
	if s := strings.TrimSpace(c.Referensi); s != "" {
		referensi = &s
	}
	if s := strings.TrimSpace(c.Catatan); s != "" {
		catatan = &s
	}
	if err := r.pool.QueryRow(ctx, sql,
		c.Nomor, c.Tanggal, c.GudangID, string(c.Tipe), c.Kategori, deskripsi,
		c.Jumlah, c.Metode, referensi, c.UserID, catatan,
	).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("insert cashflow: %w", err)
	}
	return nil
}

// GetByID lookup by id.
func (r *CashflowRepo) GetByID(ctx context.Context, id int64) (*domain.Cashflow, error) {
	sql := `SELECT ` + cashflowColumns + ` FROM cashflow WHERE id = $1`
	var c domain.Cashflow
	if err := scanCashflow(r.pool.QueryRow(ctx, sql, id), &c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCashflowNotFound
		}
		return nil, fmt.Errorf("get cashflow: %w", err)
	}
	return &c, nil
}

// Delete cashflow.
func (r *CashflowRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cashflow WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete cashflow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCashflowNotFound
	}
	return nil
}

// List paginated dengan filter.
func (r *CashflowRepo) List(ctx context.Context, f ListCashflowFilter) ([]domain.Cashflow, int, error) {
	f.Normalize()
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
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
	if f.GudangID != nil {
		conds = append(conds, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *f.GudangID)
		idx++
	}
	if t := strings.TrimSpace(f.Tipe); t != "" {
		conds = append(conds, fmt.Sprintf("tipe = $%d", idx))
		args = append(args, t)
		idx++
	}
	if k := strings.TrimSpace(f.Kategori); k != "" {
		conds = append(conds, fmt.Sprintf("kategori = $%d", idx))
		args = append(args, k)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM cashflow "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cashflow: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM cashflow %s ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		cashflowColumns, where, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query cashflow: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Cashflow, 0, f.PerPage)
	for rows.Next() {
		var c domain.Cashflow
		if err := scanCashflow(rows, &c); err != nil {
			return nil, 0, fmt.Errorf("scan cashflow: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// NextNomor generate `KAS-IN-YYYY-MM-NNNN` / `KAS-OUT-YYYY-MM-NNNN`.
func (r *CashflowRepo) NextNomor(ctx context.Context, tipe domain.CashflowTipe, tanggal time.Time) (string, error) {
	tag := "IN"
	if tipe == domain.CashflowKeluar {
		tag = "OUT"
	}
	prefix := fmt.Sprintf("KAS-%s-%04d-%02d-", tag, tanggal.Year(), int(tanggal.Month()))
	const sql = `SELECT nomor FROM cashflow
		WHERE nomor LIKE $1
		ORDER BY nomor DESC
		LIMIT 1`
	var last string
	err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return prefix + "0001", nil
	}
	if err != nil {
		return "", fmt.Errorf("next nomor cashflow: %w", err)
	}
	parts := strings.Split(last, "-")
	if len(parts) < 5 {
		return prefix + "0001", nil
	}
	var seq int
	_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &seq)
	seq++
	return fmt.Sprintf("%s%04d", prefix, seq), nil
}

// SummaryPeriode hitung total masuk/keluar/net periode.
func (r *CashflowRepo) SummaryPeriode(ctx context.Context, from, to time.Time, gudangID *int64) (domain.CashflowSummary, error) {
	conds := []string{"tanggal >= $1", "tanggal <= $2"}
	args := []any{from, to}
	idx := 3
	if gudangID != nil {
		conds = append(conds, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *gudangID)
		idx++
	}
	where := strings.Join(conds, " AND ")
	sql := fmt.Sprintf(`SELECT
		COALESCE(SUM(CASE WHEN tipe='masuk'  THEN jumlah ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN tipe='keluar' THEN jumlah ELSE 0 END), 0)
		FROM cashflow WHERE %s`, where)

	var s domain.CashflowSummary
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&s.TotalMasuk, &s.TotalKeluar); err != nil {
		return s, fmt.Errorf("summary cashflow: %w", err)
	}
	s.NetCashflow = s.TotalMasuk - s.TotalKeluar
	return s, nil
}

// KategoriBreakdown - top kategori keluar per periode.
func (r *CashflowRepo) KategoriBreakdown(ctx context.Context,
	from, to time.Time, tipe domain.CashflowTipe, gudangID *int64, limit int,
) ([]domain.CashflowKategoriBreakdown, error) {
	if limit <= 0 {
		limit = 5
	}
	conds := []string{"tanggal >= $1", "tanggal <= $2", "tipe = $3"}
	args := []any{from, to, string(tipe)}
	idx := 4
	if gudangID != nil {
		conds = append(conds, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *gudangID)
		idx++
	}
	where := strings.Join(conds, " AND ")
	sql := fmt.Sprintf(`SELECT kategori, COALESCE(SUM(jumlah), 0), COUNT(*)
		FROM cashflow WHERE %s
		GROUP BY kategori
		ORDER BY SUM(jumlah) DESC
		LIMIT $%d`, where, idx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("kategori breakdown: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CashflowKategoriBreakdown, 0, limit)
	for rows.Next() {
		var b domain.CashflowKategoriBreakdown
		if err := rows.Scan(&b.Kategori, &b.Total, &b.Count); err != nil {
			return nil, fmt.Errorf("scan kategori: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// DailyTrend - cashflow harian periode (untuk line chart).
func (r *CashflowRepo) DailyTrend(ctx context.Context,
	from, to time.Time, gudangID *int64,
) ([]domain.CashflowDailyPoint, error) {
	conds := []string{"tanggal >= $1", "tanggal <= $2"}
	args := []any{from, to}
	idx := 3
	if gudangID != nil {
		conds = append(conds, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *gudangID)
		idx++
	}
	where := strings.Join(conds, " AND ")
	sql := fmt.Sprintf(`SELECT tanggal,
			COALESCE(SUM(CASE WHEN tipe='masuk'  THEN jumlah ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tipe='keluar' THEN jumlah ELSE 0 END), 0)
		FROM cashflow WHERE %s
		GROUP BY tanggal
		ORDER BY tanggal ASC`, where)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daily trend: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CashflowDailyPoint, 0, 32)
	for rows.Next() {
		var p domain.CashflowDailyPoint
		if err := rows.Scan(&p.Tanggal, &p.Masuk, &p.Keluar); err != nil {
			return nil, fmt.Errorf("scan daily: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListKategori master list kategori.
func (r *CashflowRepo) ListKategori(ctx context.Context, tipe domain.CashflowTipe) ([]domain.CashflowKategori, error) {
	var sql string
	args := []any{}
	if tipe.IsValid() {
		sql = `SELECT id, nama, tipe FROM cashflow_kategori WHERE tipe = $1 ORDER BY nama ASC`
		args = append(args, string(tipe))
	} else {
		sql = `SELECT id, nama, tipe FROM cashflow_kategori ORDER BY tipe ASC, nama ASC`
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list kategori: %w", err)
	}
	defer rows.Close()
	out := make([]domain.CashflowKategori, 0, 16)
	for rows.Next() {
		var k domain.CashflowKategori
		var t string
		if err := rows.Scan(&k.ID, &k.Nama, &t); err != nil {
			return nil, fmt.Errorf("scan kategori: %w", err)
		}
		k.Tipe = domain.CashflowTipe(t)
		out = append(out, k)
	}
	return out, rows.Err()
}

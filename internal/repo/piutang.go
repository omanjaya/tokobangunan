package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListPiutangFilter filter list piutang per mitra.
type ListPiutangFilter struct {
	Query   string  // search nama mitra
	Aging   *string // filter bucket aging
	Page    int
	PerPage int
}

// Normalize default page/perpage.
func (f *ListPiutangFilter) Normalize() {
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

// ListInvoiceFilter filter list invoice belum lunas per mitra.
type ListInvoiceFilter struct {
	OnlyOverdue bool
	Page        int
	PerPage     int
}

// Normalize default.
func (f *ListInvoiceFilter) Normalize() {
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

// PiutangRepo akses query agregat piutang (read-only).
type PiutangRepo struct {
	pool *pgxpool.Pool
}

// NewPiutangRepo konstruktor.
func NewPiutangRepo(pool *pgxpool.Pool) *PiutangRepo {
	return &PiutangRepo{pool: pool}
}

// OutstandingByMitra hitung sisa piutang per mitra (penjualan kredit/sebagian - pembayaran).
func (r *PiutangRepo) OutstandingByMitra(ctx context.Context, mitraID int64) (int64, error) {
	const sql = `
		SELECT COALESCE((
			SELECT SUM(total) FROM penjualan
			WHERE mitra_id = $1 AND status_bayar IN ('kredit','sebagian')
		), 0) - COALESCE((
			SELECT SUM(p.jumlah) FROM pembayaran p
			JOIN penjualan pj ON pj.id = p.penjualan_id AND pj.tanggal = p.penjualan_tanggal
			WHERE p.mitra_id = $1 AND pj.status_bayar IN ('kredit','sebagian')
		), 0)`
	var v int64
	if err := r.pool.QueryRow(ctx, sql, mitraID).Scan(&v); err != nil {
		return 0, fmt.Errorf("outstanding mitra: %w", err)
	}
	if v < 0 {
		v = 0
	}
	return v, nil
}

// SummaryAll list mitra dengan piutang outstanding > 0, dengan agregat aging.
//
// Aging dihitung dari tanggal (jatuh_tempo invoice tertua, atau
// tanggal_penjualan + jatuh_tempo_hari mitra kalau jatuh_tempo NULL).
func (r *PiutangRepo) SummaryAll(ctx context.Context, f ListPiutangFilter) ([]domain.PiutangSummary, int, error) {
	f.Normalize()

	// CTE: per-invoice outstanding + due_date efektif.
	const baseCTE = `
WITH inv AS (
    SELECT pj.id AS pid, pj.tanggal AS ptg, pj.mitra_id, pj.total,
           COALESCE(pj.jatuh_tempo,
                    pj.tanggal + (m.jatuh_tempo_hari || ' days')::interval)::date AS due_date,
           COALESCE((
               SELECT SUM(jumlah) FROM pembayaran
               WHERE penjualan_id = pj.id AND penjualan_tanggal = pj.tanggal
           ), 0) AS dibayar
    FROM penjualan pj
    JOIN mitra m ON m.id = pj.mitra_id
    WHERE pj.status_bayar IN ('kredit','sebagian')
),
inv_open AS (
    SELECT * FROM inv WHERE total - dibayar > 0
),
agg AS (
    SELECT
        m.id   AS mitra_id,
        m.nama AS mitra_nama,
        m.kode AS mitra_kode,
        SUM(io.total)               AS total_penjualan,
        SUM(io.dibayar)             AS total_dibayar,
        SUM(io.total - io.dibayar)  AS outstanding,
        MIN(io.ptg)                 AS invoice_tertua,
        COUNT(*)                    AS jumlah_invoice,
        MAX(GREATEST(0, (CURRENT_DATE - io.due_date)::int)) AS max_overdue
    FROM inv_open io
    JOIN mitra m ON m.id = io.mitra_id
    WHERE m.deleted_at IS NULL
    GROUP BY m.id, m.nama, m.kode
)`

	conds := []string{"outstanding > 0"}
	args := []any{}
	idx := 1
	if q := strings.TrimSpace(f.Query); q != "" {
		conds = append(conds, fmt.Sprintf("(mitra_nama ILIKE $%d OR mitra_kode ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if f.Aging != nil && *f.Aging != "" {
		// Map bucket → range max_overdue.
		var c string
		switch *f.Aging {
		case "current":
			c = "max_overdue <= 0"
		case "1-30":
			c = "max_overdue BETWEEN 1 AND 30"
		case "31-60":
			c = "max_overdue BETWEEN 31 AND 60"
		case "61-90":
			c = "max_overdue BETWEEN 61 AND 90"
		case "90+":
			c = "max_overdue > 90"
		}
		if c != "" {
			conds = append(conds, c)
		}
	}
	where := strings.Join(conds, " AND ")

	countSQL := baseCTE + ` SELECT COUNT(*) FROM agg WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count piutang: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := baseCTE + fmt.Sprintf(`
		SELECT mitra_id, mitra_nama, mitra_kode, total_penjualan, total_dibayar,
		       outstanding, invoice_tertua, jumlah_invoice, max_overdue
		FROM agg WHERE %s
		ORDER BY max_overdue DESC, outstanding DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list piutang: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PiutangSummary, 0, f.PerPage)
	for rows.Next() {
		var s domain.PiutangSummary
		var maxOverdue int
		if err := rows.Scan(&s.MitraID, &s.MitraNama, &s.MitraKode,
			&s.TotalPenjualan, &s.TotalDibayar, &s.Outstanding,
			&s.InvoiceTertua, &s.JumlahInvoice, &maxOverdue); err != nil {
			return nil, 0, fmt.Errorf("scan piutang: %w", err)
		}
		s.Aging = domain.AgingFromDays(maxOverdue)
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// InvoicesByMitra list invoice belum lunas per mitra dengan aging.
func (r *PiutangRepo) InvoicesByMitra(ctx context.Context, mitraID int64, f ListInvoiceFilter) ([]domain.PiutangInvoice, int, error) {
	f.Normalize()

	conds := []string{"pj.mitra_id = $1", "pj.status_bayar IN ('kredit','sebagian')"}
	args := []any{mitraID}
	idx := 2
	if f.OnlyOverdue {
		// only overdue → due_date < today AND outstanding > 0; we filter post-CTE.
		_ = idx
	}

	cte := fmt.Sprintf(`
WITH inv AS (
    SELECT pj.id AS pid, pj.tanggal AS ptg, pj.nomor_kwitansi, pj.total, pj.jatuh_tempo,
           COALESCE(pj.jatuh_tempo,
                    pj.tanggal + (m.jatuh_tempo_hari || ' days')::interval)::date AS due_date,
           COALESCE((
               SELECT SUM(jumlah) FROM pembayaran
               WHERE penjualan_id = pj.id AND penjualan_tanggal = pj.tanggal
           ), 0) AS dibayar
    FROM penjualan pj
    JOIN mitra m ON m.id = pj.mitra_id
    WHERE %s
)`, strings.Join(conds, " AND "))

	postFilter := "(total - dibayar) > 0"
	if f.OnlyOverdue {
		postFilter += " AND due_date < CURRENT_DATE"
	}

	countSQL := cte + ` SELECT COUNT(*) FROM inv WHERE ` + postFilter
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count invoice piutang: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := cte + fmt.Sprintf(`
		SELECT pid, ptg, nomor_kwitansi, jatuh_tempo, total, dibayar,
		       (total - dibayar) AS outstanding,
		       GREATEST(0, (CURRENT_DATE - due_date)::int) AS hari_overdue
		FROM inv WHERE %s
		ORDER BY ptg ASC, pid ASC
		LIMIT $%d OFFSET $%d`, postFilter, len(args)+1, len(args)+2)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoice piutang: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PiutangInvoice, 0, f.PerPage)
	for rows.Next() {
		var inv domain.PiutangInvoice
		if err := rows.Scan(&inv.PenjualanID, &inv.PenjualanTanggal, &inv.NomorKwitansi,
			&inv.JatuhTempo, &inv.Total, &inv.Dibayar, &inv.Outstanding, &inv.HariOverdue); err != nil {
			return nil, 0, fmt.Errorf("scan invoice piutang: %w", err)
		}
		inv.Tanggal = inv.PenjualanTanggal
		inv.Aging = domain.AgingFromDays(inv.HariOverdue)
		out = append(out, inv)
	}
	return out, total, rows.Err()
}

// AgingBuckets agregat total outstanding per bucket aging (untuk dashboard summary).
func (r *PiutangRepo) AgingBuckets(ctx context.Context) (map[domain.PiutangAging]int64, error) {
	const sql = `
WITH inv AS (
    SELECT pj.id, pj.tanggal, pj.total,
           COALESCE(pj.jatuh_tempo,
                    pj.tanggal + (m.jatuh_tempo_hari || ' days')::interval)::date AS due_date,
           COALESCE((
               SELECT SUM(jumlah) FROM pembayaran
               WHERE penjualan_id = pj.id AND penjualan_tanggal = pj.tanggal
           ), 0) AS dibayar
    FROM penjualan pj
    JOIN mitra m ON m.id = pj.mitra_id
    WHERE pj.status_bayar IN ('kredit','sebagian') AND m.deleted_at IS NULL
)
SELECT
    SUM(CASE WHEN (CURRENT_DATE - due_date) <= 0  THEN total - dibayar ELSE 0 END) AS cur,
    SUM(CASE WHEN (CURRENT_DATE - due_date) BETWEEN 1  AND 30 THEN total - dibayar ELSE 0 END) AS b1,
    SUM(CASE WHEN (CURRENT_DATE - due_date) BETWEEN 31 AND 60 THEN total - dibayar ELSE 0 END) AS b2,
    SUM(CASE WHEN (CURRENT_DATE - due_date) BETWEEN 61 AND 90 THEN total - dibayar ELSE 0 END) AS b3,
    SUM(CASE WHEN (CURRENT_DATE - due_date) > 90 THEN total - dibayar ELSE 0 END) AS b4
FROM inv
WHERE (total - dibayar) > 0`

	var cur, b1, b2, b3, b4 *int64
	if err := r.pool.QueryRow(ctx, sql).Scan(&cur, &b1, &b2, &b3, &b4); err != nil {
		return nil, fmt.Errorf("aging buckets: %w", err)
	}
	deref := func(p *int64) int64 {
		if p == nil {
			return 0
		}
		return *p
	}
	return map[domain.PiutangAging]int64{
		domain.AgingCurrent: deref(cur),
		domain.Aging1to30:   deref(b1),
		domain.Aging31to60:  deref(b2),
		domain.Aging61to90:  deref(b3),
		domain.Aging90Plus:  deref(b4),
	}, nil
}

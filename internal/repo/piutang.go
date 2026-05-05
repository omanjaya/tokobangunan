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

// outstandingCTE returns a reusable CTE block that produces, for every open
// invoice (penjualan with status_bayar IN ('kredit','sebagian')), an
// `outstanding_real` value computed via FIFO read-time allocation of standalone
// pembayaran (penjualan_id IS NULL) per mitra.
//
// Allocation algorithm:
//   - linked     : pembayaran tied to a specific penjualan (penjualan_id NOT NULL)
//   - deposit    : SUM(pembayaran.jumlah) WHERE penjualan_id IS NULL, per mitra
//   - inv        : penjualan kredit/sebagian, sisa_after_linked = total - dibayar_linked
//   - inv_alloc  : invoices ordered by (tanggal, id) per mitra; cum_before =
//                  cumulative sisa of strictly previous invoices; the deposit
//                  pool is consumed FIFO so each invoice gets:
//                    alloc = GREATEST(0, LEAST(sisa, pool - cum_before))
//                    outstanding_real = sisa - alloc
//
// The CTE exposes columns via the `inv_alloc` table:
//
//	pid, ptg, mitra_id, total, dibayar_linked,
//	sisa_after_linked, outstanding_real, due_date, jatuh_tempo, nomor_kwitansi
//
// Callers chain extra CTEs / SELECTs after this block.
const outstandingCTE = `
WITH linked AS (
    SELECT penjualan_id, penjualan_tanggal, SUM(jumlah) AS dibayar
    FROM pembayaran
    WHERE penjualan_id IS NOT NULL
    GROUP BY penjualan_id, penjualan_tanggal
),
deposit AS (
    SELECT mitra_id, COALESCE(SUM(jumlah), 0) AS pool
    FROM pembayaran
    WHERE penjualan_id IS NULL
    GROUP BY mitra_id
),
inv AS (
    SELECT pj.id              AS pid,
           pj.tanggal          AS ptg,
           pj.mitra_id         AS mitra_id,
           pj.nomor_kwitansi   AS nomor_kwitansi,
           pj.total            AS total,
           pj.jatuh_tempo      AS jatuh_tempo,
           COALESCE(pj.jatuh_tempo,
                    pj.tanggal + (m.jatuh_tempo_hari || ' days')::interval)::date AS due_date,
           COALESCE(l.dibayar, 0) AS dibayar_linked,
           pj.total - COALESCE(l.dibayar, 0) AS sisa_after_linked
    FROM penjualan pj
    JOIN mitra m ON m.id = pj.mitra_id
    LEFT JOIN linked l
      ON l.penjualan_id = pj.id AND l.penjualan_tanggal = pj.tanggal
    WHERE pj.status_bayar IN ('kredit','sebagian')
),
inv_cum AS (
    SELECT i.*,
           COALESCE(SUM(GREATEST(i.sisa_after_linked, 0)) OVER (
               PARTITION BY i.mitra_id
               ORDER BY i.ptg ASC, i.pid ASC
               ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
           ), 0) AS cum_before
    FROM inv i
),
inv_alloc AS (
    SELECT ic.*,
           GREATEST(0,
                    ic.sisa_after_linked
                    - GREATEST(0,
                               LEAST(GREATEST(ic.sisa_after_linked, 0),
                                     COALESCE(d.pool, 0) - ic.cum_before))
           ) AS outstanding_real,
           GREATEST(0,
                    LEAST(GREATEST(ic.sisa_after_linked, 0),
                          COALESCE(d.pool, 0) - ic.cum_before)
           ) AS deposit_alloc
    FROM inv_cum ic
    LEFT JOIN deposit d ON d.mitra_id = ic.mitra_id
)`

// OutstandingByMitra hitung sisa piutang per mitra (FIFO-aware: standalone
// pembayaran dengan penjualan_id NULL ikut mengurangi outstanding).
func (r *PiutangRepo) OutstandingByMitra(ctx context.Context, mitraID int64) (int64, error) {
	sql := outstandingCTE + `
		SELECT COALESCE(SUM(outstanding_real), 0)
		FROM inv_alloc
		WHERE mitra_id = $1`
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

	// FIFO CTE produces inv_alloc with outstanding_real per invoice;
	// agg aggregates only invoices yang masih outstanding > 0.
	baseCTE := outstandingCTE + `,
inv_open AS (
    SELECT * FROM inv_alloc WHERE outstanding_real > 0
),
agg AS (
    SELECT
        m.id   AS mitra_id,
        m.nama AS mitra_nama,
        m.kode AS mitra_kode,
        SUM(io.total)                              AS total_penjualan,
        SUM(io.total - io.outstanding_real)        AS total_dibayar,
        SUM(io.outstanding_real)                   AS outstanding,
        MIN(io.ptg)                                AS invoice_tertua,
        COUNT(*)                                   AS jumlah_invoice,
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

	args := []any{mitraID}

	// FIFO allocation must be computed across all invoices for the mitra
	// before filtering — pakai outstandingCTE penuh, lalu filter mitra di SELECT.
	cte := outstandingCTE

	postFilter := "mitra_id = $1 AND outstanding_real > 0"
	if f.OnlyOverdue {
		postFilter += " AND due_date < CURRENT_DATE"
	}

	countSQL := cte + ` SELECT COUNT(*) FROM inv_alloc WHERE ` + postFilter
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count invoice piutang: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := cte + fmt.Sprintf(`
		SELECT pid, ptg, nomor_kwitansi, jatuh_tempo, total,
		       (total - outstanding_real) AS dibayar,
		       outstanding_real           AS outstanding,
		       GREATEST(0, (CURRENT_DATE - due_date)::int) AS hari_overdue
		FROM inv_alloc WHERE %s
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
	sql := outstandingCTE + `
SELECT
    SUM(CASE WHEN (CURRENT_DATE - ia.due_date) <= 0  THEN ia.outstanding_real ELSE 0 END) AS cur,
    SUM(CASE WHEN (CURRENT_DATE - ia.due_date) BETWEEN 1  AND 30 THEN ia.outstanding_real ELSE 0 END) AS b1,
    SUM(CASE WHEN (CURRENT_DATE - ia.due_date) BETWEEN 31 AND 60 THEN ia.outstanding_real ELSE 0 END) AS b2,
    SUM(CASE WHEN (CURRENT_DATE - ia.due_date) BETWEEN 61 AND 90 THEN ia.outstanding_real ELSE 0 END) AS b3,
    SUM(CASE WHEN (CURRENT_DATE - ia.due_date) > 90 THEN ia.outstanding_real ELSE 0 END) AS b4
FROM inv_alloc ia
JOIN mitra m ON m.id = ia.mitra_id
WHERE ia.outstanding_real > 0 AND m.deleted_at IS NULL`

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

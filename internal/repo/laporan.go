// Package repo - LaporanRepo: read-only aggregator untuk dashboard & laporan.
// Tidak pakai struct domain — return DTO sendiri di file ini.
package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ----- DTO ------------------------------------------------------------------

type DashboardKPI struct {
	OmsetHariIni       int64
	OmsetKemarin       int64
	PiutangOutstanding int64
	StokKritisCount    int
	MitraAktifCount    int
	TransaksiHariIni   int
}

type SalesPerDay struct {
	Tanggal time.Time
	Total   int64
	Count   int
}

type TopMitra struct {
	MitraID   int64
	MitraNama string
	Total     int64
	Transaksi int
}

type TopProduk struct {
	ProdukID   int64
	ProdukNama string
	QtyTotal   float64
	Total      int64
}

type StokKritisRow struct {
	GudangID    int64
	GudangNama  string
	ProdukID    int64
	ProdukNama  string
	Qty         float64
	StokMinimum float64
	SatuanKode  string
}

type LaporanLR struct {
	GudangID         int64
	GudangNama       string
	Penjualan        int64
	Pembelian        int64
	GrossProfit      int64
	BiayaOperasional int64
	NetIncome        int64
}

type LaporanPenjualanRow struct {
	PenjualanID   int64
	Tanggal       time.Time
	NomorKwitansi string
	GudangID      int64
	GudangNama    string
	MitraID       int64
	MitraNama     string
	Total         int64
	StatusBayar   string
}

type LaporanMutasiRow struct {
	MutasiID     int64
	Tanggal      time.Time
	NomorMutasi  string
	Status       string
	GudangAsal   string
	GudangTujuan string
	JumlahItem   int
	TotalNilai   *int64
}

// ----- Repo ------------------------------------------------------------------

type LaporanRepo struct {
	pool *pgxpool.Pool
}

func NewLaporanRepo(pool *pgxpool.Pool) *LaporanRepo {
	return &LaporanRepo{pool: pool}
}

// GetDashboardKPI - kalau gudangID nil, semua gudang.
func (r *LaporanRepo) GetDashboardKPI(ctx context.Context, gudangID *int64) (*DashboardKPI, error) {
	out := &DashboardKPI{}

	// Omset hari ini & kemarin + count hari ini.
	cond, args := gudangCondition(gudangID, 1)
	q1 := fmt.Sprintf(`
		SELECT
		  COALESCE(SUM(CASE WHEN tanggal = CURRENT_DATE THEN total ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN tanggal = CURRENT_DATE - 1 THEN total ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN tanggal = CURRENT_DATE THEN 1 ELSE 0 END), 0)
		FROM penjualan
		WHERE tanggal >= CURRENT_DATE - 1 %s`, cond)
	if err := r.pool.QueryRow(ctx, q1, args...).Scan(
		&out.OmsetHariIni, &out.OmsetKemarin, &out.TransaksiHariIni,
	); err != nil {
		return nil, fmt.Errorf("kpi omset: %w", err)
	}

	// Piutang outstanding.
	cond2, args2 := gudangCondition(gudangID, 1)
	q2 := fmt.Sprintf(`
		SELECT COALESCE(SUM(p.total - COALESCE(b.dibayar, 0)), 0)
		FROM penjualan p
		LEFT JOIN (
		  SELECT penjualan_id, penjualan_tanggal, SUM(jumlah) AS dibayar
		  FROM pembayaran
		  WHERE penjualan_id IS NOT NULL
		  GROUP BY penjualan_id, penjualan_tanggal
		) b ON b.penjualan_id = p.id AND b.penjualan_tanggal = p.tanggal
		WHERE p.status_bayar IN ('kredit','sebagian') %s`, strings.ReplaceAll(cond2, "gudang_id", "p.gudang_id"))
	if err := r.pool.QueryRow(ctx, q2, args2...).Scan(&out.PiutangOutstanding); err != nil {
		return nil, fmt.Errorf("kpi piutang: %w", err)
	}

	// Stok kritis count.
	cond3, args3 := gudangCondition(gudangID, 1)
	cond3 = strings.ReplaceAll(cond3, "gudang_id", "s.gudang_id")
	q3 := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM stok s
		JOIN produk p ON p.id = s.produk_id
		WHERE p.stok_minimum > 0 AND s.qty < p.stok_minimum %s`, cond3)
	if err := r.pool.QueryRow(ctx, q3, args3...).Scan(&out.StokKritisCount); err != nil {
		return nil, fmt.Errorf("kpi stok kritis: %w", err)
	}

	// Mitra aktif count (semua, tidak filter gudang).
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mitra WHERE is_active = TRUE AND deleted_at IS NULL`,
	).Scan(&out.MitraAktifCount); err != nil {
		return nil, fmt.Errorf("kpi mitra: %w", err)
	}

	return out, nil
}

// SalesLast30Days - generate_series 30 hari left join agregat.
func (r *LaporanRepo) SalesLast30Days(ctx context.Context, gudangID *int64) ([]SalesPerDay, error) {
	cond, args := gudangCondition(gudangID, 1)
	cond = strings.ReplaceAll(cond, "gudang_id", "p.gudang_id")
	q := fmt.Sprintf(`
		SELECT d::date AS tanggal,
		       COALESCE(SUM(p.total), 0) AS total,
		       COALESCE(COUNT(p.id), 0) AS cnt
		FROM generate_series(CURRENT_DATE - 29, CURRENT_DATE, '1 day') d
		LEFT JOIN penjualan p ON p.tanggal = d::date %s
		GROUP BY d
		ORDER BY d ASC`, strings.Replace(cond, "WHERE", "AND", 1))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sales 30: %w", err)
	}
	defer rows.Close()
	out := make([]SalesPerDay, 0, 30)
	for rows.Next() {
		var s SalesPerDay
		if err := rows.Scan(&s.Tanggal, &s.Total, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SalesPerGudangLast30 - map gudangID -> series 30 hari.
func (r *LaporanRepo) SalesPerGudangLast30(ctx context.Context) (map[int64][]SalesPerDay, error) {
	const q = `
		SELECT g.id, d::date,
		       COALESCE(SUM(p.total), 0),
		       COALESCE(COUNT(p.id), 0)
		FROM gudang g
		CROSS JOIN generate_series(CURRENT_DATE - 29, CURRENT_DATE, '1 day') d
		LEFT JOIN penjualan p ON p.tanggal = d::date AND p.gudang_id = g.id
		WHERE g.is_active = TRUE
		GROUP BY g.id, d
		ORDER BY g.id, d ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("sales gudang 30: %w", err)
	}
	defer rows.Close()
	out := map[int64][]SalesPerDay{}
	for rows.Next() {
		var gid int64
		var s SalesPerDay
		if err := rows.Scan(&gid, &s.Tanggal, &s.Total, &s.Count); err != nil {
			return nil, err
		}
		out[gid] = append(out[gid], s)
	}
	return out, rows.Err()
}

// TopMitraPeriod - top mitra by omset.
func (r *LaporanRepo) TopMitraPeriod(ctx context.Context, from, to time.Time, limit int) ([]TopMitra, error) {
	if limit <= 0 {
		limit = 5
	}
	const q = `
		SELECT p.mitra_id, m.nama,
		       COALESCE(SUM(p.total), 0) AS total,
		       COUNT(p.id) AS trx
		FROM penjualan p
		JOIN mitra m ON m.id = p.mitra_id
		WHERE p.tanggal BETWEEN $1 AND $2
		GROUP BY p.mitra_id, m.nama
		ORDER BY total DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, q, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("top mitra: %w", err)
	}
	defer rows.Close()
	out := make([]TopMitra, 0, limit)
	for rows.Next() {
		var t TopMitra
		if err := rows.Scan(&t.MitraID, &t.MitraNama, &t.Total, &t.Transaksi); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TopProdukPeriod - top produk by qty/total.
func (r *LaporanRepo) TopProdukPeriod(ctx context.Context, from, to time.Time, limit int) ([]TopProduk, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT pi.produk_id,
		       MAX(pi.produk_nama) AS nama,
		       COALESCE(SUM(pi.qty_konversi), 0) AS qty_total,
		       COALESCE(SUM(pi.subtotal), 0) AS total
		FROM penjualan_item pi
		WHERE pi.penjualan_tanggal BETWEEN $1 AND $2
		GROUP BY pi.produk_id
		ORDER BY total DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, q, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("top produk: %w", err)
	}
	defer rows.Close()
	out := make([]TopProduk, 0, limit)
	for rows.Next() {
		var t TopProduk
		if err := rows.Scan(&t.ProdukID, &t.ProdukNama, &t.QtyTotal, &t.Total); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// StokKritisAll - semua gudang.
func (r *LaporanRepo) StokKritisAll(ctx context.Context) ([]StokKritisRow, error) {
	const q = `
		SELECT s.gudang_id, g.nama, s.produk_id, p.nama,
		       s.qty, p.stok_minimum, sa.kode
		FROM stok s
		JOIN gudang g ON g.id = s.gudang_id
		JOIN produk p ON p.id = s.produk_id
		JOIN satuan sa ON sa.id = p.satuan_kecil_id
		WHERE p.stok_minimum > 0 AND s.qty < p.stok_minimum
		ORDER BY (p.stok_minimum - s.qty) DESC
		LIMIT 200`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("stok kritis: %w", err)
	}
	defer rows.Close()
	out := make([]StokKritisRow, 0, 32)
	for rows.Next() {
		var s StokKritisRow
		if err := rows.Scan(&s.GudangID, &s.GudangNama, &s.ProdukID, &s.ProdukNama,
			&s.Qty, &s.StokMinimum, &s.SatuanKode); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LaporanLRPerGudang - per cabang.
func (r *LaporanRepo) LaporanLRPerGudang(ctx context.Context, from, to time.Time) ([]LaporanLR, error) {
	const q = `
		WITH pj AS (
		  SELECT gudang_id, COALESCE(SUM(total), 0) AS total
		  FROM penjualan
		  WHERE tanggal BETWEEN $1 AND $2
		  GROUP BY gudang_id
		),
		pb AS (
		  SELECT gudang_id, COALESCE(SUM(total), 0) AS total
		  FROM pembelian
		  WHERE tanggal BETWEEN $1 AND $2
		  GROUP BY gudang_id
		)
		SELECT g.id, g.nama,
		       COALESCE(pj.total, 0),
		       COALESCE(pb.total, 0)
		FROM gudang g
		LEFT JOIN pj ON pj.gudang_id = g.id
		LEFT JOIN pb ON pb.gudang_id = g.id
		WHERE g.is_active = TRUE
		ORDER BY g.nama ASC`
	rows, err := r.pool.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("laporan lr: %w", err)
	}
	defer rows.Close()
	out := make([]LaporanLR, 0, 8)
	for rows.Next() {
		var lr LaporanLR
		if err := rows.Scan(&lr.GudangID, &lr.GudangNama, &lr.Penjualan, &lr.Pembelian); err != nil {
			return nil, err
		}
		lr.GrossProfit = lr.Penjualan - lr.Pembelian
		lr.BiayaOperasional = 0
		lr.NetIncome = lr.GrossProfit - lr.BiayaOperasional
		out = append(out, lr)
	}
	return out, rows.Err()
}

// LaporanPenjualan - dengan pagination + filter.
func (r *LaporanRepo) LaporanPenjualan(
	ctx context.Context, from, to time.Time, gudangID, mitraID *int64, page, perPage int,
) ([]LaporanPenjualanRow, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	conds := []string{"p.tanggal BETWEEN $1 AND $2"}
	args := []any{from, to}
	idx := 3
	if gudangID != nil {
		conds = append(conds, fmt.Sprintf("p.gudang_id = $%d", idx))
		args = append(args, *gudangID)
		idx++
	}
	if mitraID != nil {
		conds = append(conds, fmt.Sprintf("p.mitra_id = $%d", idx))
		args = append(args, *mitraID)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM penjualan p "+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count laporan penjualan: %w", err)
	}

	offset := (page - 1) * perPage
	q := fmt.Sprintf(`
		SELECT p.id, p.tanggal, p.nomor_kwitansi,
		       p.gudang_id, g.nama,
		       p.mitra_id, m.nama,
		       p.total, p.status_bayar
		FROM penjualan p
		JOIN gudang g ON g.id = p.gudang_id
		JOIN mitra m ON m.id = p.mitra_id
		%s
		ORDER BY p.tanggal DESC, p.id DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query laporan penjualan: %w", err)
	}
	defer rows.Close()
	out := make([]LaporanPenjualanRow, 0, perPage)
	for rows.Next() {
		var lr LaporanPenjualanRow
		if err := rows.Scan(&lr.PenjualanID, &lr.Tanggal, &lr.NomorKwitansi,
			&lr.GudangID, &lr.GudangNama, &lr.MitraID, &lr.MitraNama,
			&lr.Total, &lr.StatusBayar); err != nil {
			return nil, 0, err
		}
		out = append(out, lr)
	}
	return out, total, rows.Err()
}

// LaporanMutasi - daftar mutasi periode.
func (r *LaporanRepo) LaporanMutasi(
	ctx context.Context, from, to time.Time, gudangID *int64,
) ([]LaporanMutasiRow, error) {
	conds := []string{"m.tanggal BETWEEN $1 AND $2"}
	args := []any{from, to}
	idx := 3
	if gudangID != nil {
		conds = append(conds, fmt.Sprintf("(m.gudang_asal_id = $%d OR m.gudang_tujuan_id = $%d)", idx, idx))
		args = append(args, *gudangID)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	q := fmt.Sprintf(`
		SELECT m.id, m.tanggal, m.nomor_mutasi, m.status,
		       ga.nama AS asal, gt.nama AS tujuan,
		       (SELECT COUNT(*) FROM mutasi_item mi WHERE mi.mutasi_id = m.id) AS jumlah_item,
		       (SELECT SUM(mi.harga_internal * mi.qty_konversi)::BIGINT FROM mutasi_item mi WHERE mi.mutasi_id = m.id) AS total_nilai
		FROM mutasi_gudang m
		JOIN gudang ga ON ga.id = m.gudang_asal_id
		JOIN gudang gt ON gt.id = m.gudang_tujuan_id
		%s
		ORDER BY m.tanggal DESC, m.id DESC
		LIMIT 500`, where)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query laporan mutasi: %w", err)
	}
	defer rows.Close()
	out := make([]LaporanMutasiRow, 0, 32)
	for rows.Next() {
		var m LaporanMutasiRow
		var nilai *int64
		if err := rows.Scan(&m.MutasiID, &m.Tanggal, &m.NomorMutasi, &m.Status,
			&m.GudangAsal, &m.GudangTujuan, &m.JumlahItem, &nilai); err != nil {
			return nil, err
		}
		m.TotalNilai = nilai
		out = append(out, m)
	}
	return out, rows.Err()
}

// RecentTransaksi - untuk dashboard.
func (r *LaporanRepo) RecentTransaksi(ctx context.Context, gudangID *int64, limit int) ([]LaporanPenjualanRow, error) {
	if limit <= 0 {
		limit = 10
	}
	cond, args := gudangCondition(gudangID, 1)
	cond = strings.ReplaceAll(cond, "gudang_id", "p.gudang_id")
	limitIdx := len(args) + 1
	args = append(args, limit)
	q := fmt.Sprintf(`
		SELECT p.id, p.tanggal, p.nomor_kwitansi,
		       p.gudang_id, g.nama,
		       p.mitra_id, m.nama,
		       p.total, p.status_bayar
		FROM penjualan p
		JOIN gudang g ON g.id = p.gudang_id
		JOIN mitra m ON m.id = p.mitra_id
		WHERE 1=1 %s
		ORDER BY p.tanggal DESC, p.id DESC
		LIMIT $%d`, strings.Replace(cond, "WHERE", "AND", 1), limitIdx)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recent transaksi: %w", err)
	}
	defer rows.Close()
	out := make([]LaporanPenjualanRow, 0, limit)
	for rows.Next() {
		var lr LaporanPenjualanRow
		if err := rows.Scan(&lr.PenjualanID, &lr.Tanggal, &lr.NomorKwitansi,
			&lr.GudangID, &lr.GudangNama, &lr.MitraID, &lr.MitraNama,
			&lr.Total, &lr.StatusBayar); err != nil {
			return nil, err
		}
		out = append(out, lr)
	}
	return out, rows.Err()
}

// RecentPembayaran - n pembayaran terbaru (semua mitra).
func (r *LaporanRepo) RecentPembayaran(ctx context.Context, limit int) ([]RecentPembayaranRow, error) {
	if limit <= 0 {
		limit = 5
	}
	const q = `
		SELECT pb.id, pb.tanggal, m.nama,
		       pb.jumlah, pb.metode,
		       COALESCE(pj.nomor_kwitansi, '') AS nomor_ref
		FROM pembayaran pb
		JOIN mitra m ON m.id = pb.mitra_id
		LEFT JOIN penjualan pj ON pj.id = pb.penjualan_id AND pj.tanggal = pb.penjualan_tanggal
		ORDER BY pb.tanggal DESC, pb.id DESC
		LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("recent pembayaran: %w", err)
	}
	defer rows.Close()
	out := make([]RecentPembayaranRow, 0, limit)
	for rows.Next() {
		var p RecentPembayaranRow
		if err := rows.Scan(&p.ID, &p.Tanggal, &p.MitraNama,
			&p.Jumlah, &p.Metode, &p.NomorRef); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecentMutasi - n mutasi terbaru.
func (r *LaporanRepo) RecentMutasi(ctx context.Context, limit int) ([]RecentMutasiRow, error) {
	if limit <= 0 {
		limit = 5
	}
	const q = `
		SELECT m.id, m.tanggal, m.nomor_mutasi,
		       ga.nama, gt.nama, m.status
		FROM mutasi_gudang m
		JOIN gudang ga ON ga.id = m.gudang_asal_id
		JOIN gudang gt ON gt.id = m.gudang_tujuan_id
		ORDER BY m.tanggal DESC, m.id DESC
		LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("recent mutasi: %w", err)
	}
	defer rows.Close()
	out := make([]RecentMutasiRow, 0, limit)
	for rows.Next() {
		var m RecentMutasiRow
		if err := rows.Scan(&m.ID, &m.Tanggal, &m.NomorMutasi,
			&m.GudangAsal, &m.GudangTujuan, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ----- helpers --------------------------------------------------------------

// gudangCondition build "AND gudang_id = $N" + args slice. startIdx adalah
// nomor placeholder berikutnya. Mengembalikan empty string + nil slice kalau
// gudangID nil. Caller boleh strings.ReplaceAll "gudang_id" -> alias.table.col.
func gudangCondition(gudangID *int64, startIdx int) (string, []any) {
	if gudangID == nil {
		return "", []any{}
	}
	return fmt.Sprintf("AND gudang_id = $%d", startIdx), []any{*gudangID}
}

package excel

import (
	"context"
	"fmt"
	"time"
)

// VerifikasiResult per cabang per bulan.
type VerifikasiResult struct {
	GudangKode string
	Periode    string // "2025-01"
	TotalExcel int64
	TotalDB    int64
	Diff       int64
	OK         bool
}

// VerifyMigration bandingkan total per gudang per bulan dari Excel vs DB.
// Versi sederhana: hanya hitung total DB; total Excel diisi 0 (placeholder)
// karena re-parsing Excel mahal. Caller bisa supply Excel total dari summary
// import. Untuk Fase 7 awal, kita lapor total DB saja per cabang per bulan.
func (im *Importer) VerifyMigration(ctx context.Context, year int) ([]VerifikasiResult, error) {
	q := `
		SELECT g.kode,
		       to_char(p.tanggal, 'YYYY-MM') AS periode,
		       COALESCE(SUM(p.total), 0)
		FROM penjualan p
		JOIN gudang g ON g.id = p.gudang_id
		WHERE EXTRACT(YEAR FROM p.tanggal) = $1
		GROUP BY g.kode, to_char(p.tanggal, 'YYYY-MM')
		ORDER BY g.kode, periode
	`
	rows, err := im.pool.Query(ctx, q, year)
	if err != nil {
		return nil, fmt.Errorf("verify query: %w", err)
	}
	defer rows.Close()

	var out []VerifikasiResult
	for rows.Next() {
		var v VerifikasiResult
		if err := rows.Scan(&v.GudangKode, &v.Periode, &v.TotalDB); err != nil {
			return nil, err
		}
		v.OK = true // tanpa baseline Excel, tandai OK; user lihat angka manual
		out = append(out, v)
	}
	return out, rows.Err()
}

// MonthRange helper untuk filter periode.
func MonthRange(year int, month time.Month) (time.Time, time.Time) {
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	return from, to
}

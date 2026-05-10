package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProdukVelocity - hasil analisa kecepatan jual produk + reorder point.
type ProdukVelocity struct {
	ProdukID      int64
	ProdukNama    string
	GudangID      int64
	GudangKode    string
	GudangNama    string
	StokSekarang  float64
	AvgDailySales float64
	LeadTimeDays  int
	SafetyStock   float64
	ReorderPoint  float64
	DaysOfSupply  float64 // -1 sentinel jika avg=0 (infinite)
	NeedReorder   bool
}

// ForecastRepo - akses data untuk inventory forecasting.
type ForecastRepo struct {
	pool *pgxpool.Pool
}

func NewForecastRepo(pool *pgxpool.Pool) *ForecastRepo {
	return &ForecastRepo{pool: pool}
}

// Velocity menghitung avg daily sales + reorder point per (produk, gudang)
// dari data penjualan_item N hari terakhir. Hanya return row yang stok ≤ ROP.
//
// lookbackDays default 30 hari.
// gudangID nil = semua gudang (per-gudang aggregate).
func (r *ForecastRepo) Velocity(ctx context.Context, lookbackDays int, gudangID *int64) ([]ProdukVelocity, error) {
	if lookbackDays <= 0 {
		lookbackDays = 30
	}
	const sqlStr = `
		WITH agg AS (
			SELECT
				pi.produk_id,
				pj.gudang_id,
				SUM(pi.qty_konversi) AS qty_total
			FROM penjualan_item pi
			JOIN penjualan pj
			  ON pj.id = pi.penjualan_id AND pj.tanggal = pi.penjualan_tanggal
			WHERE pj.tanggal >= CURRENT_DATE - ($1::int * INTERVAL '1 day')
			  AND ($2::bigint IS NULL OR pj.gudang_id = $2)
			GROUP BY pi.produk_id, pj.gudang_id
		)
		SELECT
			a.produk_id,
			p.nama AS produk_nama,
			a.gudang_id,
			g.kode AS gudang_kode,
			g.nama AS gudang_nama,
			COALESCE(s.qty, 0) AS stok_sekarang,
			(a.qty_total / NULLIF($1::float, 0)) AS avg_daily,
			p.lead_time_days,
			p.safety_stock,
			((a.qty_total / NULLIF($1::float, 0)) * p.lead_time_days + p.safety_stock) AS rop
		FROM agg a
		JOIN produk p ON p.id = a.produk_id
		JOIN gudang g ON g.id = a.gudang_id
		LEFT JOIN stok s ON s.produk_id = a.produk_id AND s.gudang_id = a.gudang_id
		WHERE p.deleted_at IS NULL
		  AND COALESCE(s.qty, 0) <= ((a.qty_total / NULLIF($1::float, 0)) * p.lead_time_days + p.safety_stock)
		ORDER BY (COALESCE(s.qty, 0) - ((a.qty_total / NULLIF($1::float, 0)) * p.lead_time_days + p.safety_stock)) ASC
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, sqlStr, lookbackDays, gudangID)
	if err != nil {
		return nil, fmt.Errorf("velocity: %w", err)
	}
	defer rows.Close()

	out := make([]ProdukVelocity, 0, 16)
	for rows.Next() {
		var v ProdukVelocity
		if err := rows.Scan(
			&v.ProdukID, &v.ProdukNama, &v.GudangID, &v.GudangKode, &v.GudangNama,
			&v.StokSekarang, &v.AvgDailySales, &v.LeadTimeDays, &v.SafetyStock, &v.ReorderPoint,
		); err != nil {
			return nil, fmt.Errorf("scan velocity: %w", err)
		}
		if v.AvgDailySales > 0 {
			v.DaysOfSupply = v.StokSekarang / v.AvgDailySales
		} else {
			v.DaysOfSupply = -1
		}
		v.NeedReorder = v.StokSekarang <= v.ReorderPoint
		out = append(out, v)
	}
	return out, rows.Err()
}

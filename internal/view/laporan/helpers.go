package laporan

import (
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// netClass - color class untuk angka net income.
func netClass(v int64) string {
	if v > 0 {
		return "text-emerald-700"
	}
	if v < 0 {
		return "text-rose-700"
	}
	return "text-slate-600"
}

// sumLR - sum kolom tertentu dari semua baris LR.
func sumLR(rows []repo.LaporanLR, field string) int64 {
	var s int64
	for _, r := range rows {
		switch field {
		case "penjualan":
			s += r.Penjualan
		case "pembelian":
			s += r.Pembelian
		case "gross":
			s += r.GrossProfit
		case "biaya":
			s += r.BiayaOperasional
		case "net":
			s += r.NetIncome
		}
	}
	return s
}

// buildSparklineSVG render SVG line chart sederhana untuk daily net cashflow.
func buildSparklineSVG(points []domain.CashflowDailyPoint) string {
	if len(points) == 0 {
		return `<div class="text-sm text-slate-500">tidak ada data</div>`
	}
	const w, h = 720, 160
	const pad = 12
	var minNet, maxNet int64
	for i, p := range points {
		net := p.Masuk - p.Keluar
		if i == 0 {
			minNet, maxNet = net, net
			continue
		}
		if net < minNet {
			minNet = net
		}
		if net > maxNet {
			maxNet = net
		}
	}
	if minNet == maxNet {
		minNet -= 1
		maxNet += 1
	}
	rng := float64(maxNet - minNet)
	innerW := float64(w - 2*pad)
	innerH := float64(h - 2*pad)

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %d %d" class="w-full h-40">`, w, h))
	// zero line bila berlaku.
	if minNet < 0 && maxNet > 0 {
		zeroY := pad + innerH*(1-(0-float64(minNet))/rng)
		b.WriteString(fmt.Sprintf(`<line x1="%d" x2="%d" y1="%.1f" y2="%.1f" stroke="#e4e4e7" stroke-dasharray="3 3"/>`,
			pad, w-pad, zeroY, zeroY))
	}
	// path.
	var path strings.Builder
	for i, p := range points {
		net := p.Masuk - p.Keluar
		x := float64(pad) + innerW*float64(i)/float64(len(points)-1+1)
		if len(points) == 1 {
			x = float64(w) / 2
		}
		y := float64(pad) + innerH*(1-(float64(net)-float64(minNet))/rng)
		if i == 0 {
			path.WriteString(fmt.Sprintf("M %.1f %.1f", x, y))
		} else {
			path.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
		}
	}
	b.WriteString(fmt.Sprintf(`<path d="%s" fill="none" stroke="#0d9488" stroke-width="2"/>`, path.String()))
	b.WriteString(`</svg>`)
	return b.String()
}

// lrExportQuery - query string untuk export laporan LR.
func lrExportQuery(p LRProps) string {
	q := "?from=" + p.From + "&to=" + p.To
	return q
}

// mutasiExportQuery - query string untuk export laporan mutasi.
func mutasiExportQuery(p MutasiProps) string {
	q := "?from=" + p.From + "&to=" + p.To
	if p.GudangID > 0 {
		q += "&gudang_id=" + fmt.Sprintf("%d", p.GudangID)
	}
	return q
}

// topProdukExportQuery - query string untuk export laporan top produk.
func topProdukExportQuery(p TopProdukProps) string {
	return "?from=" + p.From + "&to=" + p.To
}

// cashflowExportQuery - query string untuk export laporan cashflow.
func cashflowExportQuery(p CashflowProps) string {
	q := "?from=" + p.From + "&to=" + p.To
	if p.GudangID > 0 {
		q += "&gudang_id=" + fmt.Sprintf("%d", p.GudangID)
	}
	return q
}

// presetChips - daftar preset periode yang digunakan pada chip filter.
type PresetChip struct {
	Key   string
	Label string
}

func PresetChipList() []PresetChip {
	return []PresetChip{
		{"today", "Hari ini"},
		{"yesterday", "Kemarin"},
		{"this_week", "Minggu ini"},
		{"this_month", "Bulan ini"},
		{"this_year", "Tahun ini"},
		{"last_month", "Bulan lalu"},
		{"last_year", "Tahun lalu"},
	}
}

// stokKritisVariant - badge variant berdasarkan rasio qty/min.
func stokKritisVariant(qty, min float64) string {
	if min <= 0 {
		return "default"
	}
	ratio := qty / min
	if ratio <= 0 {
		return "danger"
	}
	if ratio < 0.5 {
		return "danger"
	}
	return "warning"
}

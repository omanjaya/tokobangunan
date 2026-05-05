package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/tokobangunan/internal/repo"
)

// SalesChartSeries adalah satu garis pada chart.
// Label dipakai di legend, Color = warna stroke.
type SalesChartSeries struct {
	Label  string
	Color  string // tailwind/hex color (raw, dipakai di SVG stroke="...")
	GKode  string // optional kode gudang untuk preset color
	Points []repo.SalesPerDay
}

// SalesChartProps payload chart 30 hari multi-series.
// Kalau Series kosong tapi Aggregated diisi, akan dirender single line.
type SalesChartProps struct {
	Series     []SalesChartSeries
	Aggregated []repo.SalesPerDay
	// Width/Height SVG. Default 720x220 jika 0.
	Width  int
	Height int
}

// gudangColor preset warna sesuai brief.
func gudangColor(kode string) string {
	switch strings.ToUpper(kode) {
	case "CANGGU":
		return "#6366f1" // indigo
	case "SAYAN":
		return "#10b981" // emerald
	case "PEJENG":
		return "#f59e0b" // amber
	case "SAMPLANGAN":
		return "#f43f5e" // rose
	case "TEGES":
		return "#0ea5e9" // sky
	default:
		return "#71717a" // zinc
	}
}

// GudangColor exported untuk dipakai legend dan caller.
func GudangColor(kode string) string { return gudangColor(kode) }

// chartGeometry hitung dimensi efektif setelah padding.
type chartGeometry struct {
	W, H        int
	PadL, PadR  int
	PadT, PadB  int
	InnerW      int
	InnerH      int
	Days        int
	MaxY        int64
	StartDate   time.Time
}

func newChartGeometry(width, height int, days int, maxY int64, start time.Time) chartGeometry {
	if width <= 0 {
		width = 720
	}
	if height <= 0 {
		height = 220
	}
	g := chartGeometry{
		W: width, H: height,
		PadL: 56, PadR: 16, PadT: 12, PadB: 28,
		Days: days, MaxY: maxY, StartDate: start,
	}
	g.InnerW = g.W - g.PadL - g.PadR
	g.InnerH = g.H - g.PadT - g.PadB
	if g.MaxY <= 0 {
		g.MaxY = 1
	}
	return g
}

// xPos, yPos memetakan index hari & total ke koordinat SVG.
func (g chartGeometry) xPos(i int) float64 {
	if g.Days <= 1 {
		return float64(g.PadL)
	}
	return float64(g.PadL) + float64(g.InnerW)*float64(i)/float64(g.Days-1)
}
func (g chartGeometry) yPos(v int64) float64 {
	pct := float64(v) / float64(g.MaxY)
	return float64(g.PadT) + float64(g.InnerH)*(1-pct)
}

// PointsToPath convert datapoints ke SVG path "M x,y L x,y ...".
func PointsToPath(points []repo.SalesPerDay, g chartGeometry) string {
	if len(points) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range points {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.1f,%.1f ", cmd, g.xPos(i), g.yPos(p.Total))
	}
	return strings.TrimSpace(b.String())
}

// chartMaxFromSeries hitung max Total dari semua series + aggregated.
func chartMaxFromSeries(p SalesChartProps) (int64, int, time.Time) {
	var max int64
	var days int
	var start time.Time
	check := func(rows []repo.SalesPerDay) {
		if len(rows) > days {
			days = len(rows)
			start = rows[0].Tanggal
		}
		for _, r := range rows {
			if r.Total > max {
				max = r.Total
			}
		}
	}
	for _, s := range p.Series {
		check(s.Points)
	}
	check(p.Aggregated)
	if max == 0 {
		max = 1
	}
	return max, days, start
}

// FormatRupiahShort format singkat untuk Y axis: "Rp 1.2jt", "Rp 350rb".
func FormatRupiahShort(cents int64) string {
	rp := cents / 100
	neg := rp < 0
	if neg {
		rp = -rp
	}
	var s string
	switch {
	case rp >= 1_000_000_000:
		s = fmt.Sprintf("Rp %.1fM", float64(rp)/1_000_000_000)
	case rp >= 1_000_000:
		s = fmt.Sprintf("Rp %.1fjt", float64(rp)/1_000_000)
	case rp >= 1_000:
		s = fmt.Sprintf("Rp %.0frb", float64(rp)/1_000)
	default:
		s = fmt.Sprintf("Rp %d", rp)
	}
	if neg {
		return "-" + s
	}
	return s
}

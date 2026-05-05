package penjualan

import (
	"strconv"
	"strings"
	"time"

	"github.com/omanjaya/tokobangunan/internal/view/components"
)

// penjualanQueryStr build query string (tanpa "?") dari filter aktif untuk
// dipakai oleh komponen Pagination saat preserve filter.
func penjualanQueryStr(p IndexProps) string {
	parts := make([]string, 0, 5)
	if p.From != "" {
		parts = append(parts, "from="+p.From)
	}
	if p.To != "" {
		parts = append(parts, "to="+p.To)
	}
	if p.GudangID > 0 {
		parts = append(parts, "gudang_id="+strconv.FormatInt(p.GudangID, 10))
	}
	if p.Status != "" {
		parts = append(parts, "status="+p.Status)
	}
	if p.Query != "" {
		parts = append(parts, "q="+p.Query)
	}
	return strings.Join(parts, "&")
}

// penjualanPresets - quick filter chip untuk halaman /penjualan.
// Hari ini, 7 hari, bulan ini, bulan lalu, semua.
func penjualanPresets() []components.FilterPreset {
	now := time.Now()
	today := now.Format("2006-01-02")
	week := now.AddDate(0, 0, -6).Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	monthEnd := today
	prevMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	prevMonthEnd := time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	return []components.FilterPreset{
		{Key: "all", Label: "Semua", URL: "/penjualan"},
		{Key: "today", Label: "Hari ini", URL: "/penjualan?from=" + today + "&to=" + today},
		{Key: "7d", Label: "7 Hari", URL: "/penjualan?from=" + week + "&to=" + today},
		{Key: "month", Label: "Bulan ini", URL: "/penjualan?from=" + monthStart + "&to=" + monthEnd},
		{Key: "prev_month", Label: "Bulan lalu", URL: "/penjualan?from=" + prevMonthStart + "&to=" + prevMonthEnd},
	}
}

// penjualanPresetKey - kembalikan key preset yang aktif berdasarkan From/To.
// Bila tidak match preset apapun, kembalikan "all" (saat From/To kosong) atau "".
func penjualanPresetKey(p IndexProps) string {
	if p.From == "" && p.To == "" {
		return "all"
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	week := now.AddDate(0, 0, -6).Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	prevMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	prevMonthEnd := time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	switch {
	case p.From == today && p.To == today:
		return "today"
	case p.From == week && p.To == today:
		return "7d"
	case p.From == monthStart && p.To == today:
		return "month"
	case p.From == prevMonthStart && p.To == prevMonthEnd:
		return "prev_month"
	}
	return ""
}

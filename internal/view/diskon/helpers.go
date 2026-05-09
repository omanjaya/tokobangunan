// Package diskonview berisi templ component untuk modul setting/diskon.
package diskonview

import (
	"strconv"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

func idToStr(id int64) string { return strconv.FormatInt(id, 10) }

func fmtDate(t time.Time) string { return t.Format("2006-01-02") }

func fmtDatePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("2006-01-02")
}

// fmtNilai render nilai diskon human-friendly.
func fmtNilai(d domain.DiskonMaster) string {
	if d.Tipe == domain.DiskonTipePersen {
		return strconv.FormatFloat(d.Nilai, 'f', -1, 64) + "%"
	}
	return "Rp " + formatRibuan(int64(d.Nilai)/100)
}

// fmtRupiahFromCents - 200000000 cents → "2.000.000".
func fmtRupiahFromCents(cents int64) string { return formatRibuan(cents / 100) }

// fmtMaxDiskon - render *int64 cents → string atau "—".
func fmtMaxDiskon(v *int64) string {
	if v == nil {
		return "—"
	}
	return "Rp " + formatRibuan(*v/100)
}

func formatRibuan(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	// Insert "." every 3 digits from right.
	out := make([]byte, 0, len(s)+len(s)/3)
	rem := len(s) % 3
	if rem > 0 {
		out = append(out, s[:rem]...)
		if len(s) > rem {
			out = append(out, '.')
		}
	}
	for i := rem; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, '.')
		}
	}
	return string(out)
}

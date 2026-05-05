package opname

import (
	"strconv"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

func errOf(p FormProps, field string) string {
	if p.Errors == nil {
		return ""
	}
	return p.Errors[field]
}

func selisihColor(v float64) string {
	switch {
	case v > 0:
		return "text-emerald-600"
	case v < 0:
		return "text-rose-600"
	}
	return "text-slate-700"
}

func countPositiveSelisih(items []domain.StokOpnameItem) string {
	n := 0
	for _, it := range items {
		if it.Selisih > 0 {
			n++
		}
	}
	return strconv.Itoa(n)
}

func countNegativeSelisih(items []domain.StokOpnameItem) string {
	n := 0
	for _, it := range items {
		if it.Selisih < 0 {
			n++
		}
	}
	return strconv.Itoa(n)
}

// Package piutang berisi templ view untuk listing piutang aging + detail mitra.
package piutang

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - data halaman list piutang.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.PiutangSummary
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	Query string
	Aging string

	Buckets map[domain.PiutangAging]int64
}

// MitraDetailProps - detail piutang mitra + invoice.
type MitraDetailProps struct {
	Nav       layout.NavData
	User      layout.UserData
	Mitra     *domain.Mitra
	Summary   *domain.PiutangSummary
	Invoices  []domain.PiutangInvoice
	CSRFToken string
}

// AgingVariant map bucket → variant badge.
func AgingVariant(a domain.PiutangAging) string {
	switch a {
	case domain.AgingCurrent:
		return "success"
	case domain.Aging1to30:
		return "info"
	case domain.Aging31to60:
		return "warning"
	case domain.Aging61to90:
		return "warning"
	case domain.Aging90Plus:
		return "danger"
	default:
		return "default"
	}
}

// AgingLabel label tampilan untuk bucket.
func AgingLabel(a domain.PiutangAging) string {
	switch a {
	case domain.AgingCurrent:
		return "Belum jatuh tempo"
	case domain.Aging1to30:
		return "1-30 hari"
	case domain.Aging31to60:
		return "31-60 hari"
	case domain.Aging61to90:
		return "61-90 hari"
	case domain.Aging90Plus:
		return "> 90 hari"
	default:
		return string(a)
	}
}

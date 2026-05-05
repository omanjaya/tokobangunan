// Package mitra berisi templ view untuk modul Mitra: index, form, show, search.
package mitra

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - data untuk halaman list mitra.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.Mitra
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	// Filter state
	Query    string
	Tipe     string // "" = semua
	IsActive *bool  // nil = semua, true/false untuk filter

	FlashSuccess string
	CSRFToken    string
}

// FormProps - data untuk halaman create/edit mitra.
type FormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	IsEdit  bool
	Item    *domain.Mitra // nil saat create
	Errors  map[string]string
	General string // general error
}

// ShowProps - detail mitra (Fase 1: profile + 4 tab placeholder).
type ShowProps struct {
	Nav    layout.NavData
	User   layout.UserData
	Item   *domain.Mitra
	Active string // tab aktif: info | transaksi | piutang | pembayaran
}

// SearchResultsProps - partial autocomplete.
type SearchResultsProps struct {
	Items []domain.Mitra
	Query string
}

// TipeBadgeVariant memetakan tipe mitra ke warna badge.
// eceran=default(zinc), grosir=info(sky), proyek=purple(violet).
func TipeBadgeVariant(tipe string) string {
	switch tipe {
	case domain.MitraTipeGrosir:
		return "info"
	case domain.MitraTipeProyek:
		return "purple"
	case domain.MitraTipeEceran:
		fallthrough
	default:
		return "default"
	}
}

// TipeLabel label human-friendly.
func TipeLabel(tipe string) string {
	switch tipe {
	case domain.MitraTipeGrosir:
		return "Grosir"
	case domain.MitraTipeProyek:
		return "Proyek"
	case domain.MitraTipeEceran:
		return "Eceran"
	default:
		return tipe
	}
}

// StringOrDash kembalikan "—" bila pointer nil/empty.
func StringOrDash(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

// BoolPtrEq cek pointer bool sama dengan value v.
func BoolPtrEq(p *bool, v bool) bool {
	if p == nil {
		return false
	}
	return *p == v
}

// pageURL build URL untuk halaman lain dengan filter sama.
func pageURL(p IndexProps, page int) string {
	u := "/mitra?page=" + itoa(page)
	if p.Query != "" {
		u += "&q=" + urlEscape(p.Query)
	}
	if p.Tipe != "" {
		u += "&tipe=" + urlEscape(p.Tipe)
	}
	if p.IsActive != nil {
		if *p.IsActive {
			u += "&status=aktif"
		} else {
			u += "&status=nonaktif"
		}
	}
	return u
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// urlEscape minimalis: hanya escape karakter yang umum bermasalah di query.
func urlEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			out = append(out, '+')
		case c == '&' || c == '=' || c == '#' || c == '?' || c == '%' || c == '+':
			out = append(out, '%')
			out = append(out, hex(c>>4), hex(c&0x0f))
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

func hex(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'A' + b - 10
}

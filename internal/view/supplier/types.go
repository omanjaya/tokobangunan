// Package supplier berisi templ view untuk modul Supplier: index dan form.
package supplier

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - data untuk halaman list supplier.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.Supplier
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	Query    string
	IsActive *bool

	FlashSuccess string
	CSRFToken    string
}

// FormProps - data untuk halaman create/edit supplier.
type FormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	IsEdit  bool
	Item    *domain.Supplier
	Errors  map[string]string
	General string
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

// pageURL build URL paginasi dengan filter sama.
func pageURL(p IndexProps, page int) string {
	u := "/supplier?page=" + itoa(page)
	if p.Query != "" {
		u += "&q=" + urlEscape(p.Query)
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

func urlEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			out = append(out, '+')
		case c == '&' || c == '=' || c == '#' || c == '?' || c == '%' || c == '+':
			out = append(out, '%', hex(c>>4), hex(c&0x0f))
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

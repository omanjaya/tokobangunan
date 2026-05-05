package components

import (
	"strconv"
	"strings"
)

// PaginationProps - props untuk komponen Pagination reusable.
// BaseURL: path tanpa query string (e.g. "/penjualan").
// QueryStr: existing query string tanpa "?" prefix dan tanpa page/per_page,
// untuk preserve filter (e.g. "from=2025-01-01&to=2025-01-31").
type PaginationProps struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	BaseURL    string
	QueryStr   string
}

// PageURL bangun URL untuk halaman tertentu, preserve filter dari QueryStr.
func PageURL(p PaginationProps, page int) string {
	q := strings.TrimSpace(p.QueryStr)
	q = strings.TrimPrefix(q, "?")
	if q == "" {
		return p.BaseURL + "?page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(perPageOrDefault(p.PerPage))
	}
	return p.BaseURL + "?" + q + "&page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(perPageOrDefault(p.PerPage))
}

// PerPageURL bangun URL dengan per_page tertentu (reset ke page 1).
func PerPageURL(p PaginationProps, perPage int) string {
	q := strings.TrimSpace(p.QueryStr)
	q = strings.TrimPrefix(q, "?")
	if q == "" {
		return p.BaseURL + "?page=1&per_page=" + strconv.Itoa(perPage)
	}
	return p.BaseURL + "?" + q + "&page=1&per_page=" + strconv.Itoa(perPage)
}

func perPageOrDefault(n int) int {
	if n <= 0 {
		return 25
	}
	return n
}

// rangeStart hitung index awal display ("Showing X-Y of Z").
func rangeStart(p PaginationProps) int {
	if p.Total == 0 {
		return 0
	}
	return (p.Page-1)*perPageOrDefault(p.PerPage) + 1
}

func rangeEnd(p PaginationProps) int {
	end := p.Page * perPageOrDefault(p.PerPage)
	if end > p.Total {
		end = p.Total
	}
	return end
}

// formatThousand format integer dengan pemisah titik (Indonesian-style).
func formatThousand(n int) string {
	s := strconv.Itoa(n)
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte('.')
		}
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte('.')
		}
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// kvPair - hasil parse query string.
type kvPair struct {
	Key   string
	Value string
}

// parseQuery split query string menjadi pasangan key=value (URL-decoded ringan).
// Skip entry yang invalid atau key kosong.
func parseQuery(q string) []kvPair {
	q = strings.TrimSpace(q)
	q = strings.TrimPrefix(q, "?")
	if q == "" {
		return nil
	}
	out := make([]kvPair, 0, 4)
	for _, part := range strings.Split(q, "&") {
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			out = append(out, kvPair{Key: queryDecode(part), Value: ""})
			continue
		}
		k := queryDecode(part[:eq])
		v := queryDecode(part[eq+1:])
		if k == "" {
			continue
		}
		out = append(out, kvPair{Key: k, Value: v})
	}
	return out
}

// queryDecode minimal URL-decode (cukup untuk + → space dan %xx).
func queryDecode(s string) string {
	if !strings.ContainsAny(s, "+%") {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '+':
			b = append(b, ' ')
		case c == '%' && i+2 < len(s):
			h := unhex(s[i+1])<<4 | unhex(s[i+2])
			b = append(b, h)
			i += 2
		default:
			b = append(b, c)
		}
	}
	return string(b)
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// FilterPreset - quick filter chip untuk komponen FilterChips.
// Key: string match dengan current preset; Label: ditampilkan; URL: link aksi.
type FilterPreset struct {
	Key   string
	Label string
	URL   string
}

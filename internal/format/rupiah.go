// Package format menyediakan helper formatting umum (currency, tanggal, dll).
package format

import (
	"strconv"
	"strings"
)

// Rupiah memformat nilai cents (1 = Rp 0.01) menjadi "Rp 1.500.000".
// Konvensi internal aplikasi: BIGINT cents disimpan di DB, ditampilkan rupiah
// utuh tanpa desimal di UI list/form.
func Rupiah(cents int64) string {
	rupiah := cents / 100
	negative := rupiah < 0
	if negative {
		rupiah = -rupiah
	}

	s := strconv.FormatInt(rupiah, 10)
	n := len(s)
	if n <= 3 {
		if negative {
			return "Rp -" + s
		}
		return "Rp " + s
	}

	var b strings.Builder
	first := n % 3
	if first > 0 {
		b.WriteString(s[:first])
		if n > first {
			b.WriteByte('.')
		}
	}
	for i := first; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteByte('.')
		}
	}
	if negative {
		return "Rp -" + b.String()
	}
	return "Rp " + b.String()
}

// RupiahShort sama seperti Rupiah tetapi tanpa prefix "Rp ".
func RupiahShort(cents int64) string {
	full := Rupiah(cents)
	return strings.TrimPrefix(full, "Rp ")
}

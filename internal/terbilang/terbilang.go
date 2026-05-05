// Package terbilang konversi angka (rupiah utuh) ke teks Indonesia.
// Konvensi: hasil kapital di awal kalimat, diakhiri " rupiah".
package terbilang

import (
	"strings"
)

var satuan = []string{
	"", "satu", "dua", "tiga", "empat", "lima",
	"enam", "tujuh", "delapan", "sembilan",
}

// belasan untuk 11..19 (10 → "sepuluh").
func belasanWord(n int64) string {
	switch n {
	case 10:
		return "sepuluh"
	case 11:
		return "sebelas"
	default:
		return satuan[n-10] + " belas"
	}
}

// readUnder1000 mengubah 0..999 menjadi kata, tanpa "rupiah".
// Untuk seribu khusus: 1 ribu di parent, jadi readUnder1000(1) → "satu".
func readUnder1000(n int64) string {
	if n == 0 {
		return ""
	}
	parts := []string{}

	ratus := n / 100
	n %= 100
	if ratus == 1 {
		parts = append(parts, "seratus")
	} else if ratus > 1 {
		parts = append(parts, satuan[ratus]+" ratus")
	}

	if n >= 10 && n <= 19 {
		parts = append(parts, belasanWord(n))
		n = 0
	} else if n >= 20 {
		puluh := n / 10
		parts = append(parts, satuan[puluh]+" puluh")
		n %= 10
	}
	if n > 0 {
		parts = append(parts, satuan[n])
	}
	return strings.Join(parts, " ")
}

// readPositive konversi nilai > 0 ke kata (tanpa "rupiah").
func readPositive(n int64) string {
	if n == 0 {
		return "nol"
	}
	parts := []string{}

	// triliun
	if n >= 1_000_000_000_000 {
		t := n / 1_000_000_000_000
		parts = append(parts, readPositive(t)+" triliun")
		n %= 1_000_000_000_000
	}
	// miliar
	if n >= 1_000_000_000 {
		m := n / 1_000_000_000
		parts = append(parts, readPositive(m)+" miliar")
		n %= 1_000_000_000
	}
	// juta
	if n >= 1_000_000 {
		j := n / 1_000_000
		parts = append(parts, readPositive(j)+" juta")
		n %= 1_000_000
	}
	// ribu
	if n >= 1000 {
		r := n / 1000
		if r == 1 {
			parts = append(parts, "seribu")
		} else {
			parts = append(parts, readUnder1000(r)+" ribu")
		}
		n %= 1000
	}
	if n > 0 {
		parts = append(parts, readUnder1000(n))
	}
	return strings.Join(parts, " ")
}

// Konversi mengubah nilai rupiah utuh (bukan cents) menjadi kalimat terbilang.
// Contoh: 1500 → "Seribu lima ratus rupiah".
func Konversi(n int64) string {
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	body := readPositive(n)
	body = strings.TrimSpace(body)
	if body == "" {
		body = "nol"
	}
	// Capitalize first rune (ASCII).
	first := body[:1]
	rest := body[1:]
	out := strings.ToUpper(first) + rest + " rupiah"
	if negative {
		out = "Minus " + body + " rupiah"
	}
	return out
}

package excel

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	wsRegex      = regexp.MustCompile(`\s+`)
	rupiahDigits = regexp.MustCompile(`[^0-9\-,\.]`)
)

// NormalizeProdukName uppercase, trim, collapse whitespace.
// Tujuan: "Semen Portland 50kg" == "SEMEN PORTLAND 50KG" == "  Semen   Portland  50kg  ".
func NormalizeProdukName(s string) string {
	s = strings.TrimSpace(s)
	s = wsRegex.ReplaceAllString(s, " ")
	return strings.ToUpper(s)
}

// NormalizeMitraName uppercase, trim, collapse whitespace.
func NormalizeMitraName(s string) string {
	s = strings.TrimSpace(s)
	s = wsRegex.ReplaceAllString(s, " ")
	return strings.ToUpper(s)
}

// indo bulan -> month index.
var bulanMap = map[string]time.Month{
	"januari": time.January, "jan": time.January,
	"februari": time.February, "feb": time.February, "pebruari": time.February,
	"maret": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"mei": time.May,
	"juni": time.June, "jun": time.June,
	"juli": time.July, "jul": time.July,
	"agustus": time.August, "agu": time.August, "ags": time.August,
	"september": time.September, "sep": time.September, "sept": time.September,
	"oktober": time.October, "okt": time.October, "oct": time.October,
	"november": time.November, "nov": time.November, "nop": time.November,
	"desember": time.December, "des": time.December, "dec": time.December,
}

// ParseTanggal mencoba beberapa format tanggal yang umum di file Excel
// Indonesia, plus serial date Excel (angka float).
func ParseTanggal(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("tanggal kosong")
	}

	// Excel serial date (number).
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1 && f < 80000 {
		// Excel epoch 1899-12-30 (windows leap-year bug compatible).
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.Add(time.Duration(f * 24 * float64(time.Hour))), nil
	}

	// Format umum.
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-1-2006",
		"02-01-06",
		"2-1-06",
		"02/01/06",
		"2/1/06",
		"02.01.2006",
		"02.01.06",
		"2006/01/02",
		"01/02/2006",
		"01-02-06",
		"1-2-06",
		"01/02/06",
	}
	for _, fmtStr := range formats {
		if t, err := time.Parse(fmtStr, s); err == nil {
			return t, nil
		}
	}

	// Format "DD MMMM YYYY" (e.g. "5 Februari 2025").
	parts := strings.Fields(strings.ToLower(s))
	if len(parts) == 3 {
		day, errDay := strconv.Atoi(parts[0])
		year, errYear := strconv.Atoi(parts[2])
		mon, ok := bulanMap[parts[1]]
		if errDay == nil && errYear == nil && ok {
			return time.Date(year, mon, day, 0, 0, 0, 0, time.UTC), nil
		}
	}

	return time.Time{}, fmt.Errorf("format tanggal tidak dikenal: %q", s)
}

// ParseRupiah mengubah string seperti "Rp 1.234.567,50" atau "1234567" ke
// cents (int64). Tanda koma dianggap pemisah desimal (Indonesia), titik
// dianggap pemisah ribuan.
func ParseRupiah(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	s = rupiahDigits.ReplaceAllString(s, "")
	if s == "" || s == "-" {
		return 0, nil
	}

	// Heuristik: kalau ada koma, anggap koma=desimal, titik=thousands.
	// Kalau tidak ada koma dan ada titik, bisa juga titik=desimal (Excel default
	// AS). Cek: kalau ada titik tunggal dengan 1-2 digit setelahnya, anggap desimal.
	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	switch {
	case hasComma && hasDot:
		// Indonesia style: titik thousands, koma desimal (Rp 1.234.567,50)
		// US style: koma thousands, titik desimal (1,234,567.50)
		// Heuristik: posisi terakhir koma vs titik. Yang terakhir = desimal.
		lastComma := strings.LastIndex(s, ",")
		lastDot := strings.LastIndex(s, ".")
		if lastComma > lastDot {
			// Indonesia style
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			// US style
			s = strings.ReplaceAll(s, ",", "")
		}
	case hasComma:
		// Heuristik: kalau ada multiple koma atau digit setelah koma terakhir
		// = 3, anggap thousands separator (US/Excel default).
		multiComma := strings.Count(s, ",") > 1
		idx := strings.LastIndex(s, ",")
		after := len(s) - idx - 1
		if multiComma || after == 3 {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			// Likely Indonesia decimal (1-2 digit setelah koma).
			s = strings.Replace(s, ",", ".", 1)
		}
	case hasDot:
		// Bisa thousands atau desimal. Heuristik: kalau hanya 1 titik dan
		// digit setelahnya 1-2 → desimal; selainnya thousands.
		idx := strings.Index(s, ".")
		after := len(s) - idx - 1
		multiDot := strings.Count(s, ".") > 1
		if multiDot || after == 3 {
			s = strings.ReplaceAll(s, ".", "")
		}
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse rupiah %q: %w", s, err)
	}
	cents := int64(f * 100)
	if negative {
		cents = -cents
	}
	// Hasilnya kembali ke unit "rupiah utuh" (BIGINT cents convention pakai
	// rupiah-as-unit). Schema toko bangunan menyimpan total dalam rupiah penuh
	// (tidak ada cents), jadi kita kembalikan int64 dari nilai utuh; pemanggil
	// boleh tafsirkan sesuai konteks. Untuk konsistensi, kalau parse di atas
	// 100x, kita kembalikan dibagi 100. Karena kita tidak tahu apakah Excel
	// memang punya digit desimal, kita defensif: rupiah Indonesia jarang punya
	// pecahan, jadi return int64(f).
	return int64(f), nil
}

// ParseQty mem-parse jumlah numerik (qty) dari string. Bisa "10", "10,5",
// "10/5.5" (rumus Excel sudah dievaluasi excelize, tapi defensif).
func ParseQty(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// kalau ada slash, evaluasi pembagian.
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) == 2 {
			a, errA := parseFloatLoose(parts[0])
			b, errB := parseFloatLoose(parts[1])
			if errA == nil && errB == nil && b != 0 {
				return a / b, nil
			}
		}
	}
	return parseFloatLoose(s)
}

func parseFloatLoose(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.Replace(s, ",", ".", 1)
	return strconv.ParseFloat(s, 64)
}

// IsHeaderCell heuristik untuk identifikasi sel header. True kalau berisi
// kata kunci umum (Tgl, Tanggal, Mitra, Item, Qty, Harga, Total, dll).
func IsHeaderCell(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	keywords := []string{
		"tgl", "tanggal", "mitra", "item", "produk", "qty", "harga",
		"total", "subtotal", "satuan", "no", "nama", "saldo", "jumlah",
		"pembayaran", "metode", "stok",
	}
	for _, k := range keywords {
		if s == k {
			return true
		}
	}
	return false
}

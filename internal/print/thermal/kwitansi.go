// Package thermal generate ESC/POS byte stream untuk printer thermal POS
// (58mm dan 80mm). Format text-only fixed width.
package thermal

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/terbilang"
)

// Kontrol kode ESC/POS umum.
const (
	escInit      = "\x1b@"      // initialize printer
	escCut       = "\x1d\x56\x00" // partial cut
	escCondensed = "\x1b\x21\x01" // small font (font B)
	escNormal    = "\x1b\x21\x00" // normal font
	escBoldOn    = "\x1b\x45\x01"
	escBoldOff   = "\x1b\x45\x00"
	escAlignCtr  = "\x1b\x61\x01"
	escAlignLeft = "\x1b\x61\x00"
	escAlignRgt  = "\x1b\x61\x02"
)

// Generate58mm output ESC/POS untuk printer thermal 58mm (~32 char width).
func Generate58mm(p *domain.Penjualan, mitra *domain.Mitra, gudang *domain.Gudang, tokoInfo *domain.TokoInfo) ([]byte, error) {
	return renderKwitansi(p, mitra, gudang, tokoInfo, 32)
}

// Generate80mm output ESC/POS untuk printer thermal 80mm (~48 char width).
func Generate80mm(p *domain.Penjualan, mitra *domain.Mitra, gudang *domain.Gudang, tokoInfo *domain.TokoInfo) ([]byte, error) {
	return renderKwitansi(p, mitra, gudang, tokoInfo, 48)
}

func renderKwitansi(
	p *domain.Penjualan, mitra *domain.Mitra, gudang *domain.Gudang, tokoInfo *domain.TokoInfo, width int,
) ([]byte, error) {
	if p == nil || mitra == nil || gudang == nil {
		return nil, fmt.Errorf("thermal: arg nil")
	}
	// Resolve kop: tokoInfo override gudang bila tersedia.
	kopNama := gudang.Nama
	var kopAlamat, kopTelepon string
	if gudang.Alamat != nil {
		kopAlamat = *gudang.Alamat
	}
	if gudang.Telepon != nil {
		kopTelepon = *gudang.Telepon
	}
	if tokoInfo != nil && strings.TrimSpace(tokoInfo.Nama) != "" {
		kopNama = tokoInfo.Nama
		if tokoInfo.Alamat != "" {
			kopAlamat = tokoInfo.Alamat
		}
		if tokoInfo.Telepon != "" {
			kopTelepon = tokoInfo.Telepon
		}
	}

	var b bytes.Buffer

	b.WriteString(escInit)
	b.WriteString(escCondensed)
	sep := strings.Repeat("=", width) + "\n"
	dash := strings.Repeat("-", width) + "\n"

	// Header kop (centered, bold).
	b.WriteString(sep)
	b.WriteString(escAlignCtr)
	b.WriteString(escBoldOn)
	b.WriteString(truncateLine(kopNama, width) + "\n")
	b.WriteString(escBoldOff)
	if kopAlamat != "" {
		b.WriteString(wrapAndCenter(kopAlamat, width))
	}
	if kopTelepon != "" {
		b.WriteString(truncateLine("Telp: "+kopTelepon, width) + "\n")
	}
	if tokoInfo != nil && tokoInfo.NPWP != "" {
		b.WriteString(truncateLine("NPWP: "+tokoInfo.NPWP, width) + "\n")
	}
	b.WriteString(escAlignLeft)
	b.WriteString(sep)

	// No & tanggal.
	b.WriteString(truncateLine("No: "+p.NomorKwitansi, width) + "\n")
	b.WriteString(truncateLine("Tgl: "+p.Tanggal.Format("02/01/2006"), width) + "\n")
	b.WriteString(dash)

	// Mitra.
	b.WriteString(wrap("Nama: "+mitra.Nama, width))
	b.WriteString("\n")

	// Header tabel item.
	b.WriteString(headerCols(width))
	b.WriteString(dash)

	// Items.
	for _, it := range p.Items {
		b.WriteString(wrap(it.ProdukNama, width))
		qtyStr := fmtQty(it.Qty) + " " + it.SatuanKode
		hargaStr := format.RupiahShort(it.HargaSatuan)
		totalStr := format.RupiahShort(it.Subtotal)
		line := fmt.Sprintf("  %s x %s", qtyStr, hargaStr)
		b.WriteString(twoCol(line, totalStr, width))
	}
	b.WriteString(dash)

	// Totals.
	b.WriteString(twoCol("SUBTOTAL", format.RupiahShort(p.Subtotal), width))
	b.WriteString(twoCol("DISKON", format.RupiahShort(p.Diskon), width))
	b.WriteString(escBoldOn)
	b.WriteString(twoCol("TOTAL", format.RupiahShort(p.Total), width))
	b.WriteString(escBoldOff)
	b.WriteString("\n")

	// Terbilang.
	b.WriteString("Terbilang:\n")
	terb := terbilang.Konversi(p.Total/100) + " rupiah"
	b.WriteString(wrap(terb, width))
	b.WriteString("\n")

	// Status.
	b.WriteString("Status: " + strings.ToUpper(string(p.StatusBayar)) + "\n")
	b.WriteString(sep)

	// Footer thanks.
	b.WriteString(escAlignCtr)
	b.WriteString("Terima kasih\n")
	b.WriteString(escAlignLeft)
	b.WriteString(sep)

	// Feed lines + cut.
	b.WriteString("\n\n\n\n")
	b.WriteString(escCut)
	return b.Bytes(), nil
}

// headerCols menyusun header tabel sesuai lebar.
func headerCols(width int) string {
	if width >= 48 {
		return fmt.Sprintf("%-30s %6s %10s\n", "ITEM", "QTY", "TOTAL")
	}
	return fmt.Sprintf("%-16s %5s %9s\n", "ITEM", "QTY", "TOTAL")
}

// twoCol render kiri rata kiri, kanan rata kanan dalam 1 baris.
func twoCol(left, right string, width int) string {
	if len(left)+1+len(right) <= width {
		pad := width - len(left) - len(right)
		return left + strings.Repeat(" ", pad) + right + "\n"
	}
	// fallback: dua baris.
	return left + "\n" + strings.Repeat(" ", maxInt(0, width-len(right))) + right + "\n"
}

// wrap memecah teks panjang jadi multi-line dengan width chars.
func wrap(s string, width int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var out strings.Builder
	for len(s) > width {
		// Cari spasi terakhir sebelum width.
		cut := width
		if i := strings.LastIndexByte(s[:width], ' '); i > 0 {
			cut = i
		}
		out.WriteString(s[:cut])
		out.WriteByte('\n')
		s = strings.TrimLeft(s[cut:], " ")
	}
	out.WriteString(s)
	out.WriteByte('\n')
	return out.String()
}

// wrapAndCenter wrap lalu setiap baris di-center secara visual.
// Untuk thermal printer alignment dilakukan via ESC/POS, jadi cukup wrap.
func wrapAndCenter(s string, width int) string {
	return wrap(s, width)
}

func truncateLine(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "."
}

func fmtQty(q float64) string {
	// Hilangkan trailing zero.
	s := fmt.Sprintf("%.4f", q)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Package escpos generate ESC/P byte stream untuk printer dot matrix
// (Epson LX-310 dan kompatibel).
package escpos

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/terbilang"
)

// Kontrol kode ESC/P.
const (
	escInit      = "\x1b@"    // initialize printer
	escCondensed = "\x1b\x0f" // condensed mode (17 cpi)
	escDouble    = "\x1b\x47" // double-strike on
	escDoubleOff = "\x1b\x48" // double-strike off
	formFeed     = "\x0c"
)

// width karakter untuk layout fixed-width (condensed 17cpi pada 8" → ~136 col,
// safe pakai 80 untuk kertas standar 9.5" carbon).
const lineWidth = 68

// GenerateKwitansiESCP render kwitansi ke byte stream ESC/P plain.
// Layout text-only fixed-width; bisa di-pipe langsung ke printer.
func GenerateKwitansiESCP(
	p *domain.Penjualan,
	mitra *domain.Mitra,
	gudang *domain.Gudang,
	tokoInfo *domain.TokoInfo,
) ([]byte, error) {
	// Resolve kop: toko_info > gudang.
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

	sep := strings.Repeat("=", lineWidth) + "\n"
	dash := strings.Repeat("-", lineWidth) + "\n"

	b.WriteString(sep)
	b.WriteString(escDouble)
	b.WriteString(centerLine(kopNama))
	b.WriteString(escDoubleOff)
	if kopAlamat != "" {
		b.WriteString(centerLine(kopAlamat))
	}
	if kopTelepon != "" {
		b.WriteString(centerLine("Telp: " + kopTelepon))
	}
	if tokoInfo != nil && tokoInfo.NPWP != "" {
		b.WriteString(centerLine("NPWP: " + tokoInfo.NPWP))
	}
	b.WriteString(sep)
	b.WriteString("\n")

	// Header detail.
	tanggal := p.Tanggal.Format("02 Jan 2006")
	left := "No: " + p.NomorKwitansi
	right := "Tanggal: " + tanggal
	b.WriteString(twoColumn(left, right) + "\n")
	b.WriteString("\n")

	b.WriteString(formatField("Sudah terima dari", mitra.Nama))
	b.WriteString(formatField("Uang sejumlah", format.Rupiah(p.Total)))
	terb := terbilang.Konversi(p.Total / 100)
	b.WriteString(formatField("Terbilang", terb))
	b.WriteString(formatField("Untuk pembayaran", "Pembelian barang"))
	b.WriteString("\n")

	// Tabel item.
	b.WriteString(dash)
	b.WriteString(fmt.Sprintf("%-3s %-28s %6s %-4s %10s %12s\n",
		"No", "Item", "Qty", "Sat", "Harga", "Total"))
	b.WriteString(dash)

	for i, it := range p.Items {
		b.WriteString(fmt.Sprintf("%-3d %-28s %6.2f %-4s %10s %12s\n",
			i+1,
			truncate(it.ProdukNama, 28),
			it.Qty,
			truncate(it.SatuanKode, 4),
			format.RupiahShort(it.HargaSatuan),
			format.RupiahShort(it.Subtotal),
		))
	}
	b.WriteString(dash)

	// Footer total (rata kanan).
	b.WriteString(rightAlign("Subtotal: " + format.RupiahShort(p.Subtotal)))
	b.WriteString(rightAlign("Diskon:   " + format.RupiahShort(p.Diskon)))
	b.WriteString(escDouble)
	b.WriteString(rightAlign("TOTAL:    " + format.RupiahShort(p.Total)))
	b.WriteString(escDoubleOff)
	b.WriteString("\n\n")

	// Tanda tangan (rata kanan dengan offset).
	tempat := gudang.Nama
	if comma := strings.IndexByte(tempat, ','); comma > 0 {
		tempat = tempat[:comma]
	}
	b.WriteString(indentLeft(40, fmt.Sprintf("%s, %s", tempat, tanggal)))
	b.WriteString(indentLeft(40, "Penerima,"))
	b.WriteString("\n\n\n")
	b.WriteString(indentLeft(40, "(_______________________)"))
	b.WriteString("\n")

	b.WriteString(formFeed)
	return b.Bytes(), nil
}

// ----- helpers ---------------------------------------------------------------

func centerLine(s string) string {
	if len(s) >= lineWidth {
		return s + "\n"
	}
	pad := (lineWidth - len(s)) / 2
	return strings.Repeat(" ", pad) + s + "\n"
}

func twoColumn(left, right string) string {
	if len(left)+len(right)+1 >= lineWidth {
		return left + " " + right
	}
	pad := lineWidth - len(left) - len(right)
	return left + strings.Repeat(" ", pad) + right
}

func formatField(label, value string) string {
	const labelW = 18
	if len(label) > labelW {
		label = label[:labelW]
	}
	return fmt.Sprintf("%-*s : %s\n", labelW, label, value)
}

func rightAlign(s string) string {
	if len(s) >= lineWidth {
		return s + "\n"
	}
	return strings.Repeat(" ", lineWidth-len(s)) + s + "\n"
}

func indentLeft(offset int, s string) string {
	return strings.Repeat(" ", offset) + s + "\n"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "."
}

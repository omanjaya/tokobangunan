// Package pdf - label produk dengan barcode Code128.
package pdf

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/jung-kurt/gofpdf"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// LabelInfo data ringkas untuk render label.
type LabelInfo struct {
	SKU       string
	Nama      string
	HargaCent int64 // 0 = jangan tampilkan harga
}

// GenerateLabelProdukPDF render label produk berisi barcode + SKU + nama + harga.
// Layout grid 2 kolom × 10 baris di kertas A4 portrait. count = jumlah label cetak.
func GenerateLabelProdukPDF(p *domain.Produk, info LabelInfo, count int) ([]byte, error) {
	if count <= 0 {
		count = 1
	}
	if count > 200 {
		count = 200
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(8, 8, 8)
	pdf.SetAutoPageBreak(true, 8)
	pdf.AddPage()

	// Generate barcode PNG (Code128) dari SKU. Reuse hasil untuk semua label.
	bc, err := code128.Encode(info.SKU)
	if err != nil {
		return nil, fmt.Errorf("encode barcode: %w", err)
	}
	scaled, err := barcode.Scale(bc, 300, 60)
	if err != nil {
		return nil, fmt.Errorf("scale barcode: %w", err)
	}
	var bcBuf bytes.Buffer
	if err := png.Encode(&bcBuf, scaled); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	imgName := "barcode-" + info.SKU
	pdf.RegisterImageOptionsReader(imgName, gofpdf.ImageOptions{ImageType: "PNG"}, &bcBuf)

	// Layout: 2 cols, 10 rows per page → 20 label per A4.
	const cols, rows = 2, 10
	const labelW, labelH = 95.0, 27.0
	const gapX, gapY = 4.0, 1.0
	startX, startY := 8.0, 8.0

	idx := 0
	for i := 0; i < count; i++ {
		col := idx % cols
		row := (idx / cols) % rows
		if idx > 0 && col == 0 && row == 0 {
			pdf.AddPage()
		}
		x := startX + float64(col)*(labelW+gapX)
		y := startY + float64(row)*(labelH+gapY)

		pdf.SetDrawColor(220, 220, 220)
		pdf.Rect(x, y, labelW, labelH, "D")

		// Nama produk
		pdf.SetXY(x+2, y+1.5)
		pdf.SetFont("Arial", "B", 8)
		nama := info.Nama
		if len(nama) > 60 {
			nama = nama[:60] + "…"
		}
		pdf.CellFormat(labelW-4, 4, nama, "", 0, "L", false, 0, "")

		// Barcode image
		pdf.ImageOptions(imgName, x+2, y+6, labelW-4, 12,
			false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")

		// SKU
		pdf.SetXY(x+2, y+18.5)
		pdf.SetFont("Courier", "", 8)
		pdf.CellFormat(labelW-4, 3.5, info.SKU, "", 0, "C", false, 0, "")

		// Harga (kalau ada)
		if info.HargaCent > 0 {
			pdf.SetXY(x+2, y+22)
			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(labelW-4, 4, formatRupiahCents(info.HargaCent), "", 0, "C", false, 0, "")
		}

		idx++
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// formatRupiahCents format cents → "Rp 12.500".
func formatRupiahCents(cents int64) string {
	rupiah := cents / 100
	s := fmt.Sprintf("%d", rupiah)
	// Insert separator titik dari kanan tiap 3 digit.
	n := len(s)
	if n <= 3 {
		return "Rp " + s
	}
	out := make([]byte, 0, n+n/3)
	pre := n % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
		if pre < n {
			out = append(out, '.')
		}
	}
	for i := pre; i < n; i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < n {
			out = append(out, '.')
		}
	}
	return "Rp " + string(out)
}

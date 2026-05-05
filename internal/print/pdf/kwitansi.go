// Package pdf generate PDF kwitansi A5 untuk modul penjualan.
package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/terbilang"
)

// GenerateKwitansiA5 render kwitansi penjualan ke PDF A5 portrait.
// watermark: "ASLI" / "TEMBUSAN" / "" (kosong = tidak digambar).
// tokoInfo (boleh nil) override kop dengan data app_setting toko_info.
// Kalau tokoInfo nil atau Nama kosong, fallback ke data gudang.
func GenerateKwitansiA5(
	p *domain.Penjualan,
	mitra *domain.Mitra,
	gudang *domain.Gudang,
	tokoInfo *domain.TokoInfo,
	watermark string,
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
	// A5 = 148 × 210 mm.
	pdf := gofpdf.New("P", "mm", "A5", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()

	// Header: kop toko.
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 6, kopNama, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	if kopAlamat != "" {
		pdf.CellFormat(0, 4, kopAlamat, "", 1, "C", false, 0, "")
	}
	if kopTelepon != "" {
		pdf.CellFormat(0, 4, "Telp: "+kopTelepon, "", 1, "C", false, 0, "")
	}
	if tokoInfo != nil && tokoInfo.NPWP != "" {
		pdf.CellFormat(0, 4, "NPWP: "+tokoInfo.NPWP, "", 1, "C", false, 0, "")
	}
	pdf.Ln(2)
	pdf.SetLineWidth(0.4)
	x1, y1 := pdf.GetXY()
	pdf.Line(10, y1, 138, y1)
	pdf.Ln(3)

	// Box "KWITANSI" + nomor + tempat & tanggal.
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 7, "KWITANSI", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(40, 5, "No. Kwitansi", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, p.NomorKwitansi, "", 1, "L", false, 0, "")

	pdf.CellFormat(40, 5, "Tanggal", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, p.Tanggal.Format("02 January 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Detail mitra & jumlah.
	pdf.CellFormat(40, 5, "Sudah terima dari", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 5, mitra.Nama, "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)

	pdf.CellFormat(40, 5, "Uang sejumlah", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, format.Rupiah(p.Total), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)

	pdf.CellFormat(40, 5, "Terbilang", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "I", 9)
	terb := terbilang.Konversi(p.Total / 100)
	pdf.MultiCell(0, 5, terb, "", "L", false)
	pdf.SetFont("Arial", "", 9)

	pdf.CellFormat(40, 5, "Untuk pembayaran", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Pembelian barang", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Tabel item.
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(8, 6, "No", "1", 0, "C", true, 0, "")
	pdf.CellFormat(50, 6, "Nama Produk", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 6, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 6, "Sat", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 6, "Harga", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 6, "Total", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 8)
	for i, it := range p.Items {
		pdf.CellFormat(8, 5, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 5, truncate(it.ProdukNama, 30), "1", 0, "L", false, 0, "")
		pdf.CellFormat(15, 5, fmt.Sprintf("%g", it.Qty), "1", 0, "R", false, 0, "")
		pdf.CellFormat(15, 5, it.SatuanKode, "1", 0, "C", false, 0, "")
		pdf.CellFormat(20, 5, format.RupiahShort(it.HargaSatuan), "1", 0, "R", false, 0, "")
		pdf.CellFormat(20, 5, format.RupiahShort(it.Subtotal), "1", 1, "R", false, 0, "")
	}

	// Footer subtotal/diskon/total.
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(88, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(20, 5, "Subtotal", "", 0, "R", false, 0, "")
	pdf.CellFormat(20, 5, format.RupiahShort(p.Subtotal), "", 1, "R", false, 0, "")

	pdf.CellFormat(88, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(20, 5, "Diskon", "", 0, "R", false, 0, "")
	pdf.CellFormat(20, 5, format.RupiahShort(p.Diskon), "", 1, "R", false, 0, "")

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(88, 6, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(20, 6, "TOTAL", "T", 0, "R", false, 0, "")
	pdf.CellFormat(20, 6, format.RupiahShort(p.Total), "T", 1, "R", false, 0, "")
	pdf.Ln(4)

	// Tanda tangan.
	pdf.SetFont("Arial", "", 9)
	tempat := gudang.Nama
	if comma := strings.IndexByte(tempat, ','); comma > 0 {
		tempat = tempat[:comma]
	}
	pdf.CellFormat(80, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, fmt.Sprintf("%s, %s", tempat, p.Tanggal.Format("02 Jan 2006")), "", 1, "L", false, 0, "")
	pdf.CellFormat(80, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Penerima,", "", 1, "L", false, 0, "")
	pdf.Ln(12)
	pdf.CellFormat(80, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "(_______________)", "", 1, "L", false, 0, "")

	// Watermark di tengah.
	if w := strings.TrimSpace(watermark); w != "" {
		pdf.SetTextColor(220, 220, 220)
		pdf.SetFont("Arial", "B", 56)
		pdf.TransformBegin()
		pdf.TransformRotate(45, 74, 105)
		pdf.SetXY(20, 95)
		pdf.CellFormat(108, 20, strings.ToUpper(w), "", 0, "C", false, 0, "")
		pdf.TransformEnd()
		pdf.SetTextColor(0, 0, 0)
	}
	_ = x1

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("output pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

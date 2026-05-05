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

// GenerateFakturPajak render Faktur Pajak A4 portrait. Tidak e-faktur resmi —
// hanya tanda terima manual untuk transaksi B2B PKP.
func GenerateFakturPajak(
	p *domain.Penjualan,
	mitra *domain.Mitra,
	gudang *domain.Gudang,
	pajak *domain.PajakConfig,
	tokoInfo *domain.TokoInfo,
) ([]byte, error) {
	if p == nil {
		return nil, fmt.Errorf("penjualan nil")
	}
	if p.PPNAmount <= 0 {
		return nil, fmt.Errorf("penjualan tidak memiliki PPN")
	}

	// Resolve identitas penjual.
	namaPenjual := gudang.Nama
	var alamatPenjual, npwpPenjual string
	if gudang.Alamat != nil {
		alamatPenjual = *gudang.Alamat
	}
	if tokoInfo != nil && strings.TrimSpace(tokoInfo.Nama) != "" {
		namaPenjual = tokoInfo.Nama
		if tokoInfo.Alamat != "" {
			alamatPenjual = tokoInfo.Alamat
		}
		npwpPenjual = tokoInfo.NPWP
	}
	if pajak != nil && pajak.PKP {
		if pajak.NamaPKP != "" {
			namaPenjual = pajak.NamaPKP
		}
		if pajak.AlamatPKP != "" {
			alamatPenjual = pajak.AlamatPKP
		}
		if pajak.NPWPPKP != "" {
			npwpPenjual = pajak.NPWPPKP
		}
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 12, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Header.
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 7, "FAKTUR PAJAK", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, "(Tanda Terima Manual — bukan e-Faktur resmi DJP)", "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Nomor seri & tanggal.
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(35, 5, "Nomor Seri", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 5, "FP/"+p.NomorKwitansi, "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(35, 5, "Tanggal", "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, p.Tanggal.Format("02 January 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Penjual.
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Pengusaha Kena Pajak (Penjual)", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	fakturRow(pdf, "Nama", namaPenjual)
	if alamatPenjual != "" {
		fakturRow(pdf, "Alamat", alamatPenjual)
	}
	if npwpPenjual != "" {
		fakturRow(pdf, "NPWP", npwpPenjual)
	}
	pdf.Ln(2)

	// Pembeli.
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Pembeli Barang/Jasa Kena Pajak", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	fakturRow(pdf, "Nama", mitra.Nama)
	if mitra.Alamat != nil && *mitra.Alamat != "" {
		fakturRow(pdf, "Alamat", *mitra.Alamat)
	}
	if mitra.NPWP != nil && *mitra.NPWP != "" {
		fakturRow(pdf, "NPWP", *mitra.NPWP)
	}
	pdf.Ln(3)

	// Tabel item dengan DPP.
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(229, 231, 235)
	cols := []struct {
		title string
		w     float64
		align string
	}{
		{"No", 10, "C"},
		{"Nama BKP/JKP", 70, "L"},
		{"Qty", 18, "R"},
		{"Harga Satuan", 30, "R"},
		{"DPP (Rp)", 32, "R"},
	}
	for _, col := range cols {
		pdf.CellFormat(col.w, 6, col.title, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for i, it := range p.Items {
		// DPP per row = subtotal item (sudah dikurangi diskon item).
		pdf.CellFormat(cols[0].w, 6, fmt.Sprintf("%d", i+1), "1", 0, cols[0].align, false, 0, "")
		pdf.CellFormat(cols[1].w, 6, truncate(it.ProdukNama, 38), "1", 0, cols[1].align, false, 0, "")
		pdf.CellFormat(cols[2].w, 6, fmt.Sprintf("%g %s", it.Qty, it.SatuanKode), "1", 0, cols[2].align, false, 0, "")
		pdf.CellFormat(cols[3].w, 6, format.RupiahShort(it.HargaSatuan), "1", 0, cols[3].align, false, 0, "")
		pdf.CellFormat(cols[4].w, 6, format.RupiahShort(it.Subtotal), "1", 0, cols[4].align, false, 0, "")
		pdf.Ln(-1)
	}

	// Footer rekap pajak.
	labelW := cols[0].w + cols[1].w + cols[2].w + cols[3].w
	valW := cols[4].w

	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(labelW, 6, "Subtotal", "1", 0, "R", false, 0, "")
	pdf.CellFormat(valW, 6, format.RupiahShort(p.Subtotal), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.CellFormat(labelW, 6, "Diskon", "1", 0, "R", false, 0, "")
	pdf.CellFormat(valW, 6, format.RupiahShort(p.Diskon), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(labelW, 6, "Dasar Pengenaan Pajak (DPP)", "1", 0, "R", false, 0, "")
	pdf.CellFormat(valW, 6, format.RupiahShort(p.DPP), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.CellFormat(labelW, 6,
		fmt.Sprintf("PPN %.1f%%", p.PPNPersen), "1", 0, "R", false, 0, "")
	pdf.CellFormat(valW, 6, format.RupiahShort(p.PPNAmount), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(243, 244, 246)
	pdf.CellFormat(labelW, 7, "TOTAL (DPP + PPN)", "1", 0, "R", true, 0, "")
	pdf.CellFormat(valW, 7, format.RupiahShort(p.Total), "1", 0, "R", true, 0, "")
	pdf.Ln(-1)

	// Terbilang.
	pdf.Ln(2)
	pdf.SetFont("Arial", "I", 9)
	pdf.MultiCell(0, 5, "Terbilang: "+terbilang.Konversi(p.Total/100), "", "L", false)

	// Tanda tangan.
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	tempat := gudang.Nama
	if comma := strings.IndexByte(tempat, ','); comma > 0 {
		tempat = tempat[:comma]
	}
	pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, fmt.Sprintf("%s, %s", tempat, p.Tanggal.Format("02 Jan 2006")), "", 1, "L", false, 0, "")
	pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Penjual,", "", 1, "L", false, 0, "")
	pdf.Ln(15)
	pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "(_______________________)", "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("output faktur pajak: %w", err)
	}
	return buf.Bytes(), nil
}

func fakturRow(pdf *gofpdf.Fpdf, label, value string) {
	pdf.CellFormat(35, 5, label, "", 0, "L", false, 0, "")
	pdf.CellFormat(2, 5, ":", "", 0, "L", false, 0, "")
	pdf.MultiCell(0, 5, value, "", "L", false)
}

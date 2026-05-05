package excel

import (
	"strings"
	"time"
)

// PenjualanRow representasi 1 baris transaksi penjualan dari sheet DETAIL.
type PenjualanRow struct {
	SourceFile string
	SourceSh   string
	RowIdx     int

	GudangKode string
	Tanggal    time.Time
	MitraNama  string
	ProdukNama string
	Qty        float64
	Satuan     string
	Harga      int64
	Total      int64
	Catatan    string
}

// MutasiRow representasi transfer antar gudang.
type MutasiRow struct {
	SourceFile string
	SourceSh   string
	RowIdx     int

	Tanggal       time.Time
	GudangAsal    string
	GudangTujuan  string
	ProdukNama    string
	Qty           float64
	HargaInternal int64
	Total         int64
}

// PiutangAwal opening balance per mitra.
type PiutangAwal struct {
	GudangKode string
	MitraNama  string
	Saldo      int64
	Tanggal    time.Time
}

// PembayaranRow record pembayaran customer.
type PembayaranRow struct {
	GudangKode string
	Tanggal    time.Time
	MitraNama  string
	Jumlah     int64
	Metode     string
	Referensi  string
}

// StokRow stok awal per produk per gudang.
type StokRow struct {
	GudangKode string
	ProdukNama string
	Qty        float64
	Satuan     string
}

// PembelianRow record hutang/pembelian dari supplier.
type PembelianRow struct {
	GudangKode string
	Tanggal    time.Time
	Supplier   string
	ProdukNama string
	Qty        float64
	Satuan     string
	Harga      int64
	Total      int64
}

// TabunganRow setor/tarik tabungan mitra.
type TabunganRow struct {
	GudangKode string
	Tanggal    time.Time
	MitraNama  string
	Debit      int64
	Kredit     int64
	Catatan    string
}

// ParseMitraMain menstreaming sheet MAIN → PenjualanRow.
//
// MAIN layout (per inspeksi langsung):
//
//	A(0)=Tanggal, B(1)=Bulan/Tahun, C(2)=ITEM, D(3)=IN, E(4)=OUT,
//	F(5)=HPP, G(6)=HJ (harga jual), H(7)=Sisa Stock, I(8)=L/R,
//	J(9)=Penjualan (total Rp), K(10)=Stat (BON/CASH),
//	L(11)=Nama (mitra), M(12)=Bon, N(13)=Nominal, O(14)=STATUS.
//
// Hanya baris dengan E (qty OUT) > 0 yang dianggap penjualan.
// Header di row 1, data dari row 3+.
func ParseMitraMain(wb *Workbook, sheet, gudangKode string) ([]PenjualanRow, []Anomaly, error) {
	var (
		rows  []PenjualanRow
		anoms []Anomaly
	)
	err := wb.StreamRows(sheet, func(idx int, row []string) error {
		if idx < 3 {
			return nil
		}
		if isRowEmpty(row) || len(row) < 12 {
			return nil
		}
		tglStr := getCol(row, 0)
		produk := strings.TrimSpace(getCol(row, 2))
		outStr := getCol(row, 4)
		hargaStr := getCol(row, 6)
		totalStr := getCol(row, 9)
		statKredit := strings.TrimSpace(getCol(row, 10)) // BON / CASH
		mitra := strings.TrimSpace(getCol(row, 11))
		statusBayar := strings.TrimSpace(getCol(row, 14))

		if mitra == "" || produk == "" {
			return nil
		}
		tgl, err := ParseTanggal(tglStr)
		if err != nil {
			anoms = append(anoms, Anomaly{
				File: wb.Path(), Sheet: sheet, RowIdx: idx,
				Reason: "tanggal tidak valid: " + tglStr,
			})
			return nil
		}
		qty, err := ParseQty(outStr)
		if err != nil || qty <= 0 {
			// Bukan penjualan (qty OUT 0 → ini barangkali pembelian / saldo awal).
			return nil
		}
		harga, _ := ParseRupiah(hargaStr)
		total, _ := ParseRupiah(totalStr)
		if total == 0 {
			total = int64(qty * float64(harga))
		}

		// Status bayar dari kolom STATUS atau dari Stat.
		status := "lunas"
		switch strings.ToUpper(statusBayar) {
		case "BON", "BLM LUNAS", "BELUM LUNAS", "KREDIT":
			status = "kredit"
		case "LUNAS":
			status = "lunas"
		default:
			if strings.EqualFold(statKredit, "BON") {
				status = "kredit"
			}
		}

		rows = append(rows, PenjualanRow{
			SourceFile: wb.Path(),
			SourceSh:   sheet,
			RowIdx:     idx,
			GudangKode: gudangKode,
			Tanggal:    tgl,
			MitraNama:  mitra,
			ProdukNama: produk,
			Qty:        qty,
			Harga:      harga,
			Total:      total,
			Catatan:    status,
		})
		return nil
	})
	return rows, anoms, err
}

// ParseAntarGudang membaca file Antar Gudang.xlsx dengan struktur pivot:
// setiap sheet (Canggu, Sayan, dll) berisi multiple route blocks. Setiap
// block: kolom Tgl|Route|Qty|Harga|Total. Block dimulai di row 3 (header).
func ParseAntarGudang(wb *Workbook) ([]MutasiRow, []Anomaly, error) {
	var (
		mutasi []MutasiRow
		anoms  []Anomaly
	)
	for _, sheet := range wb.Sheets() {
		if strings.EqualFold(sheet, "Total") {
			continue
		}
		gudangAsal := strings.ToUpper(sheet)
		all, err := wb.Rows(sheet)
		if err != nil {
			return nil, nil, err
		}
		if len(all) < 4 {
			continue
		}

		// Header row 3 (idx 2). Identifikasi posisi block: cari kolom dengan
		// pola "<asal>-<tujuan>". Setiap match → block sebanyak 5 kolom.
		header := all[2]
		type block struct {
			startCol int
			tujuan   string
		}
		var blocks []block
		for col, cell := range header {
			cell = strings.TrimSpace(cell)
			if cell == "" {
				continue
			}
			parts := strings.SplitN(cell, "-", 2)
			if len(parts) == 2 {
				asal := strings.ToUpper(strings.TrimSpace(parts[0]))
				tujuan := strings.ToUpper(strings.TrimSpace(parts[1]))
				if asal == gudangAsal {
					// Block dimulai di col-1 (Tgl), tapi excel layout:
					// Tgl | <asal>-<tujuan> | Qty | Harga | Total
					blocks = append(blocks, block{startCol: col - 1, tujuan: tujuan})
				}
			}
		}

		for rowIdx := 3; rowIdx < len(all); rowIdx++ {
			row := all[rowIdx]
			for _, b := range blocks {
				tglStr := getCol(row, b.startCol)
				produkStr := getCol(row, b.startCol+1)
				qtyStr := getCol(row, b.startCol+2)
				hargaStr := getCol(row, b.startCol+3)
				totalStr := getCol(row, b.startCol+4)

				if strings.TrimSpace(produkStr) == "" || strings.TrimSpace(qtyStr) == "" {
					continue
				}
				if strings.Contains(strings.ToLower(tglStr), "total") {
					continue
				}
				tgl, err := ParseTanggal(tglStr)
				if err != nil {
					anoms = append(anoms, Anomaly{
						File: wb.Path(), Sheet: sheet, RowIdx: rowIdx + 1,
						Reason: "tgl tidak valid: " + tglStr,
					})
					continue
				}
				qty, err := ParseQty(qtyStr)
				if err != nil || qty <= 0 {
					continue
				}
				harga, _ := ParseRupiah(hargaStr)
				total, _ := ParseRupiah(totalStr)
				if total == 0 {
					total = int64(qty * float64(harga))
				}
				mutasi = append(mutasi, MutasiRow{
					SourceFile:    wb.Path(),
					SourceSh:      sheet,
					RowIdx:        rowIdx + 1,
					Tanggal:       tgl,
					GudangAsal:    gudangAsal,
					GudangTujuan:  b.tujuan,
					ProdukNama:    strings.TrimSpace(produkStr),
					Qty:           qty,
					HargaInternal: harga,
					Total:         total,
				})
			}
		}
	}
	return mutasi, anoms, nil
}

// ParsePiutang membaca sheet PIUTANG.
// Layout (inspeksi): A=NAMA, B=UTANG (saldo), C=SISA HUTANG (status),
// D-F=detail pembayaran lain. Header di row 3, data dari row 5+.
func ParsePiutang(wb *Workbook, sheet, gudangKode string, openingDate time.Time) ([]PiutangAwal, error) {
	var rows []PiutangAwal
	all, err := wb.Rows(sheet)
	if err != nil {
		return nil, err
	}
	for idx, row := range all {
		if idx < 4 {
			continue
		}
		mitra := strings.TrimSpace(getCol(row, 0))
		if mitra == "" || IsHeaderCell(mitra) ||
			strings.EqualFold(mitra, "Row Labels") ||
			strings.EqualFold(mitra, "Grand Total") {
			continue
		}
		saldoStr := getCol(row, 1)
		saldo, _ := ParseRupiah(saldoStr)
		if saldo <= 0 {
			continue
		}
		rows = append(rows, PiutangAwal{
			GudangKode: gudangKode,
			MitraNama:  mitra,
			Saldo:      saldo,
			Tanggal:    openingDate,
		})
	}
	return rows, nil
}

// ParsePembayaran sheet Pembayaran.
// Layout (inspeksi): A=Tanggal, B=Nama, C=Piutang Awal, D=Pembayaran,
// E=Keterangan, F=Baki Debet. Data dari row 3+.
func ParsePembayaran(wb *Workbook, sheet, gudangKode string) ([]PembayaranRow, []Anomaly, error) {
	var (
		rows  []PembayaranRow
		anoms []Anomaly
	)
	all, err := wb.Rows(sheet)
	if err != nil {
		return nil, nil, err
	}
	for idx, row := range all {
		if idx < 2 || isRowEmpty(row) {
			continue
		}
		tglStr := strings.TrimSpace(getCol(row, 0))
		mitra := strings.TrimSpace(getCol(row, 1))
		jumStr := getCol(row, 3) // Pembayaran (kolom D)
		if mitra == "" || IsHeaderCell(mitra) {
			continue
		}
		// Skip silently kalau tgl kosong (draft row Excel).
		if tglStr == "" {
			continue
		}
		tgl, err := ParseTanggal(tglStr)
		if err != nil {
			anoms = append(anoms, Anomaly{
				File: wb.Path(), Sheet: sheet, RowIdx: idx + 1,
				Reason: "tgl: " + tglStr,
			})
			continue
		}
		jum, err := ParseRupiah(jumStr)
		if err != nil || jum <= 0 {
			continue
		}
		rows = append(rows, PembayaranRow{
			GudangKode: gudangKode,
			Tanggal:    tgl,
			MitraNama:  mitra,
			Jumlah:     jum,
			Metode:     "tunai",
			Referensi:  strings.TrimSpace(getCol(row, 4)), // Keterangan
		})
	}
	return rows, anoms, nil
}

// ParseStokGudang sheet Stok Gudang.
// Layout (inspeksi): A=ITEM, B=Stok Awal, C=IN, D=OUT, E=Stok Akhir,
// F=Barang Rijek, G=HPP, H=JUMLAH. Data dari row 3+. Pakai kolom E (Stok Akhir).
func ParseStokGudang(wb *Workbook, sheet, gudangKode string) ([]StokRow, error) {
	var rows []StokRow
	all, err := wb.Rows(sheet)
	if err != nil {
		return nil, err
	}
	for idx, row := range all {
		if idx < 2 {
			continue
		}
		nama := strings.TrimSpace(getCol(row, 0))
		if nama == "" || IsHeaderCell(nama) {
			continue
		}
		qtyStr := getCol(row, 4) // Stok Akhir
		qty, err := ParseQty(qtyStr)
		if err != nil {
			continue
		}
		rows = append(rows, StokRow{
			GudangKode: gudangKode,
			ProdukNama: nama,
			Qty:        qty,
		})
	}
	return rows, nil
}

// ParseHutang sheet Hutang. A=Tgl, B=Supplier, C=Item, D=Qty, E=Satuan,
// F=Harga, G=Total.
func ParseHutang(wb *Workbook, sheet, gudangKode string) ([]PembelianRow, []Anomaly, error) {
	var (
		rows  []PembelianRow
		anoms []Anomaly
	)
	all, err := wb.Rows(sheet)
	if err != nil {
		return nil, nil, err
	}
	for idx, row := range all {
		if idx < 2 || isRowEmpty(row) {
			continue
		}
		tglStr := getCol(row, 0)
		supplier := strings.TrimSpace(getCol(row, 1))
		produk := strings.TrimSpace(getCol(row, 2))
		if supplier == "" || produk == "" || IsHeaderCell(supplier) {
			continue
		}
		tgl, err := ParseTanggal(tglStr)
		if err != nil {
			anoms = append(anoms, Anomaly{
				File: wb.Path(), Sheet: sheet, RowIdx: idx + 1,
				Reason: "tgl: " + tglStr,
			})
			continue
		}
		qty, _ := ParseQty(getCol(row, 3))
		harga, _ := ParseRupiah(getCol(row, 5))
		total, _ := ParseRupiah(getCol(row, 6))
		if total == 0 {
			total = int64(qty * float64(harga))
		}
		rows = append(rows, PembelianRow{
			GudangKode: gudangKode,
			Tanggal:    tgl,
			Supplier:   supplier,
			ProdukNama: produk,
			Qty:        qty,
			Satuan:     strings.TrimSpace(getCol(row, 4)),
			Harga:      harga,
			Total:      total,
		})
	}
	return rows, anoms, nil
}

// ParseTabungan sheet Tabungan. A=Tgl, B=Mitra, C=Debit, D=Kredit, E=Catatan.
func ParseTabungan(wb *Workbook, sheet, gudangKode string) ([]TabunganRow, []Anomaly, error) {
	var (
		rows  []TabunganRow
		anoms []Anomaly
	)
	all, err := wb.Rows(sheet)
	if err != nil {
		return nil, nil, err
	}
	for idx, row := range all {
		if idx < 2 || isRowEmpty(row) {
			continue
		}
		tglStr := getCol(row, 0)
		mitra := strings.TrimSpace(getCol(row, 1))
		if mitra == "" || IsHeaderCell(mitra) {
			continue
		}
		tgl, err := ParseTanggal(tglStr)
		if err != nil {
			anoms = append(anoms, Anomaly{
				File: wb.Path(), Sheet: sheet, RowIdx: idx + 1,
				Reason: "tgl: " + tglStr,
			})
			continue
		}
		debit, _ := ParseRupiah(getCol(row, 2))
		kredit, _ := ParseRupiah(getCol(row, 3))
		if debit <= 0 && kredit <= 0 {
			continue
		}
		rows = append(rows, TabunganRow{
			GudangKode: gudangKode,
			Tanggal:    tgl,
			MitraNama:  mitra,
			Debit:      debit,
			Kredit:     kredit,
			Catatan:    strings.TrimSpace(getCol(row, 4)),
		})
	}
	return rows, anoms, nil
}

func isRowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

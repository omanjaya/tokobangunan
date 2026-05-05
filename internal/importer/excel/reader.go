// Package excel berisi importer untuk membaca file Excel (.xlsx) dari sistem
// lama (Mitra Usaha + Antar Gudang) dan menormalkannya ke skema PostgreSQL.
//
// Modul ini one-shot, bukan bagian dari runtime aplikasi. Dipanggil dari
// cmd/migrate-excel via flag CLI.
package excel

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// Workbook membungkus *excelize.File supaya kita bisa kontrol API yang
// dipakai (Rows + StreamRows) tanpa membocorkan dependency excelize ke caller.
type Workbook struct {
	path string
	file *excelize.File
}

// OpenWorkbook membuka file .xlsx untuk dibaca.
func OpenWorkbook(path string) (*Workbook, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx %s: %w", path, err)
	}
	return &Workbook{path: path, file: f}, nil
}

// Path mengembalikan path file sumber.
func (w *Workbook) Path() string { return w.path }

// Close menutup workbook.
func (w *Workbook) Close() error {
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

// Sheets mengembalikan daftar nama sheet sesuai urutan di workbook.
func (w *Workbook) Sheets() []string {
	return w.file.GetSheetList()
}

// Rows membaca seluruh sheet ke memory sebagai slice of slice string.
// Cocok untuk sheet kecil (PIUTANG, Pembayaran, dll). Untuk sheet besar
// seperti DETAIL Canggu (96K rows), pakai StreamRows.
func (w *Workbook) Rows(sheet string) ([][]string, error) {
	rows, err := w.file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("get rows %s: %w", sheet, err)
	}
	return rows, nil
}

// StreamRows membaca sheet baris demi baris tanpa load semuanya ke memory.
// Index dimulai dari 1 (sesuai konvensi Excel).
func (w *Workbook) StreamRows(sheet string, fn func(rowIdx int, row []string) error) error {
	it, err := w.file.Rows(sheet)
	if err != nil {
		return fmt.Errorf("open row iterator %s: %w", sheet, err)
	}
	defer it.Close()

	idx := 0
	for it.Next() {
		idx++
		cols, err := it.Columns()
		if err != nil {
			return fmt.Errorf("read row %d: %w", idx, err)
		}
		if err := fn(idx, cols); err != nil {
			return err
		}
	}
	if err := it.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}
	return nil
}

// CountRows menghitung jumlah baris non-kosong di sheet (cepat, streaming).
func (w *Workbook) CountRows(sheet string) (int, error) {
	count := 0
	err := w.StreamRows(sheet, func(_ int, row []string) error {
		if len(row) == 0 {
			return nil
		}
		// Anggap baris kosong jika semua sel kosong.
		for _, c := range row {
			if c != "" {
				count++
				return nil
			}
		}
		return nil
	})
	return count, err
}

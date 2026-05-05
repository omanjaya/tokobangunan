package excel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileInfo metadata file Excel di direktori sumber.
type FileInfo struct {
	Path     string
	Name     string
	Size     int64
	Sheets   []string
	RowCount map[string]int
}

// SayanComparison hasil bandingkan SAYAN.xlsx vs SAYAN(1).xlsx.
type SayanComparison struct {
	File1            string
	File2            string
	HashFile1        string
	HashFile2        string
	RowCountSheet1   int
	RowCountSheet2   int
	IdenticalContent bool
	Recommendation   string // "use SAYAN" / "use SAYAN(1)" / "manual review"
}

// ProdukCandidate kandidat master produk dari hasil scan DETAIL.
type ProdukCandidate struct {
	Nama       string // sudah dinormalisasi (UPPER + collapse ws)
	NamaAsli   string // first occurrence raw
	Occurrence int
	Satuan     map[string]int // satuan -> count
}

// MitraCandidate kandidat master mitra.
type MitraCandidate struct {
	Nama       string
	NamaAsli   string
	Occurrence int
	Cabang     map[string]int // cabang -> count
}

// Anomaly catatan baris yang aneh.
type Anomaly struct {
	File   string
	Sheet  string
	RowIdx int
	Reason string
}

// AuditReport ringkasan audit.
type AuditReport struct {
	SourceDir        string
	Files            []FileInfo
	SayanComparison  *SayanComparison
	ProdukCandidates []ProdukCandidate
	MitraCandidates  []MitraCandidate
	KonversiFactors  []float64
	Anomalies        []Anomaly
}

// AuditAll memindai folder sumber, return report.
func AuditAll(sourceDir string) (*AuditReport, error) {
	report := &AuditReport{SourceDir: sourceDir}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read source dir: %w", err)
	}

	produkAgg := map[string]*ProdukCandidate{}
	mitraAgg := map[string]*MitraCandidate{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".xlsx") {
			continue
		}
		full := filepath.Join(sourceDir, e.Name())
		fi, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		info := FileInfo{
			Path:     full,
			Name:     e.Name(),
			Size:     fi.Size(),
			RowCount: map[string]int{},
		}

		wb, err := OpenWorkbook(full)
		if err != nil {
			report.Anomalies = append(report.Anomalies, Anomaly{
				File: e.Name(), Reason: "gagal buka file: " + err.Error(),
			})
			report.Files = append(report.Files, info)
			continue
		}
		info.Sheets = wb.Sheets()
		for _, sh := range info.Sheets {
			n, _ := wb.CountRows(sh)
			info.RowCount[sh] = n
		}

		// Hanya scan produk/mitra dari file MITRA USAHA <CABANG>.
		// Sheet MAIN adalah master transaction log (96K+ rows untuk CANGGU).
		if strings.Contains(strings.ToUpper(e.Name()), "MITRA USAHA") {
			cabang := detectCabang(e.Name())
			if hasSheet(info.Sheets, "MAIN") {
				_ = scanProdukDariMain(wb, "MAIN", produkAgg)
				_ = scanMitraDariMain(wb, "MAIN", cabang, mitraAgg)
			}
		}
		_ = wb.Close()
		report.Files = append(report.Files, info)
	}

	// Bandingkan SAYAN files.
	var sayan, sayan1 string
	for _, f := range report.Files {
		up := strings.ToUpper(f.Name)
		switch {
		case strings.Contains(up, "SAYAN") && strings.Contains(up, "(1)"):
			sayan1 = f.Path
		case strings.Contains(up, "SAYAN"):
			sayan = f.Path
		}
	}
	if sayan != "" && sayan1 != "" {
		cmp, err := CompareSayan(sayan, sayan1)
		if err == nil {
			report.SayanComparison = cmp
		} else {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Reason: "compare sayan: " + err.Error(),
			})
		}
	}

	// Sort kandidat by occurrence DESC.
	for _, p := range produkAgg {
		report.ProdukCandidates = append(report.ProdukCandidates, *p)
	}
	for _, m := range mitraAgg {
		report.MitraCandidates = append(report.MitraCandidates, *m)
	}
	sort.Slice(report.ProdukCandidates, func(i, j int) bool {
		return report.ProdukCandidates[i].Occurrence > report.ProdukCandidates[j].Occurrence
	})
	sort.Slice(report.MitraCandidates, func(i, j int) bool {
		return report.MitraCandidates[i].Occurrence > report.MitraCandidates[j].Occurrence
	})

	report.KonversiFactors = []float64{5.5, 4.0} // ditemukan di rumus Antar Gudang

	return report, nil
}

func hasSheet(sheets []string, name string) bool {
	for _, s := range sheets {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

func detectCabang(filename string) string {
	up := strings.ToUpper(filename)
	for _, c := range []string{"CANGGU", "SAYAN", "PEJENG", "SAMPLANGAN", "TEGES"} {
		if strings.Contains(up, c) {
			return c
		}
	}
	return "UNKNOWN"
}

// CompareSayan hitung hash file & row count sheet DETAIL.
func CompareSayan(file1, file2 string) (*SayanComparison, error) {
	h1, err := fileHash(file1)
	if err != nil {
		return nil, err
	}
	h2, err := fileHash(file2)
	if err != nil {
		return nil, err
	}

	wb1, err := OpenWorkbook(file1)
	if err != nil {
		return nil, err
	}
	defer wb1.Close()
	wb2, err := OpenWorkbook(file2)
	if err != nil {
		return nil, err
	}
	defer wb2.Close()

	n1, _ := wb1.CountRows("MAIN")
	n2, _ := wb2.CountRows("MAIN")

	cmp := &SayanComparison{
		File1: file1, File2: file2,
		HashFile1: h1, HashFile2: h2,
		RowCountSheet1: n1, RowCountSheet2: n2,
		IdenticalContent: h1 == h2,
	}
	switch {
	case h1 == h2:
		cmp.Recommendation = "identical content; pakai SAYAN saja, hapus (1)"
	case n1 > n2:
		cmp.Recommendation = "SAYAN.xlsx lebih banyak baris → gunakan SAYAN"
	case n2 > n1:
		cmp.Recommendation = "SAYAN (1).xlsx lebih banyak baris → gunakan SAYAN_1"
	default:
		cmp.Recommendation = "row count sama tapi konten beda → review manual"
	}
	return cmp, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MAIN sheet layout (per inspeksi langsung):
// A=Tanggal, B=Bulan/Tahun, C=ITEM (produk), D=IN (qty masuk),
// E=OUT (qty keluar=penjualan), F=HPP, G=HJ (harga jual),
// H=Sisa Stock, I=L/R, J=Penjualan (total Rp),
// K=Stat (BON/CASH/dll), L=Nama (mitra), M=Bon, N=Nominal, O=STATUS.
// Header di row 1, data dari row 3+ (row 2 sering empty).

// scanProdukDariMain mengumpulkan distinct nama ITEM dari MAIN.
func scanProdukDariMain(wb *Workbook, sheet string, agg map[string]*ProdukCandidate) error {
	return wb.StreamRows(sheet, func(idx int, row []string) error {
		if idx < 3 || len(row) < 3 {
			return nil
		}
		raw := strings.TrimSpace(getCol(row, 2))
		if raw == "" || IsHeaderCell(raw) {
			return nil
		}
		// Skip baris yang nampak angka-only (footer/sum).
		if isNumeric(raw) {
			return nil
		}
		key := NormalizeProdukName(raw)
		if key == "" {
			return nil
		}
		entry, ok := agg[key]
		if !ok {
			entry = &ProdukCandidate{
				Nama: key, NamaAsli: raw,
				Satuan: map[string]int{},
			}
			agg[key] = entry
		}
		entry.Occurrence++
		return nil
	})
}

// scanMitraDariMain mengumpulkan distinct Nama (kolom L) dari MAIN.
// Hanya baris dengan OUT > 0 (penjualan) yang dianggap mitra.
func scanMitraDariMain(wb *Workbook, sheet, cabang string, agg map[string]*MitraCandidate) error {
	return wb.StreamRows(sheet, func(idx int, row []string) error {
		if idx < 3 || len(row) < 12 {
			return nil
		}
		out, _ := ParseQty(getCol(row, 4))
		if out <= 0 {
			return nil
		}
		raw := strings.TrimSpace(getCol(row, 11))
		if raw == "" || IsHeaderCell(raw) {
			return nil
		}
		if isNumeric(raw) {
			return nil
		}
		key := NormalizeMitraName(raw)
		if key == "" {
			return nil
		}
		entry, ok := agg[key]
		if !ok {
			entry = &MitraCandidate{
				Nama: key, NamaAsli: raw,
				Cabang: map[string]int{},
			}
			agg[key] = entry
		}
		entry.Occurrence++
		entry.Cabang[cabang]++
		return nil
	})
}

func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, c := range s {
		if c == '0' || c == '1' || c == '2' || c == '3' || c == '4' ||
			c == '5' || c == '6' || c == '7' || c == '8' || c == '9' ||
			c == ',' || c == '.' || c == '-' || c == ' ' {
			continue
		}
		return false
	}
	return true
}

func getCol(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

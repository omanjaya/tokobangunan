package handler

import (
	"context"
	"encoding/csv"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// csvImportResult - hasil import CSV produk.
type csvImportResult struct {
	Imported int
	Failed   int
	ErrMsgs  []string
}

// importProdukCSV parse CSV dan create produk via service.
// Format: SKU, Nama, Kategori, Satuan (header opsional).
func importProdukCSV(
	ctx context.Context,
	raw string,
	produkSvc *service.ProdukService,
	satuanRepo *repo.SatuanRepo,
) csvImportResult {
	res := csvImportResult{}
	if strings.TrimSpace(raw) == "" {
		return res
	}

	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	records, _ := r.ReadAll()

	startIdx := 0
	if len(records) > 0 {
		first := strings.ToLower(strings.Join(records[0], ","))
		if strings.Contains(first, "sku") || strings.Contains(first, "nama") {
			startIdx = 1
		}
	}

	for i := startIdx; i < len(records); i++ {
		row := records[i]
		if len(row) < 2 {
			continue
		}
		sku := strings.TrimSpace(row[0])
		nama := strings.TrimSpace(row[1])
		if sku == "" || nama == "" {
			continue
		}
		kategori := ""
		if len(row) > 2 {
			kategori = strings.TrimSpace(row[2])
		}
		satKode := "PCS"
		if len(row) > 3 && strings.TrimSpace(row[3]) != "" {
			satKode = strings.ToUpper(strings.TrimSpace(row[3]))
		}
		sat, err := satuanRepo.GetByKode(ctx, satKode)
		if err != nil {
			all, _ := satuanRepo.List(ctx)
			if len(all) == 0 {
				res.Failed++
				appendCapped(&res.ErrMsgs, sku+": tidak ada master satuan")
				continue
			}
			sat = &all[0]
		}
		if _, err := produkSvc.Create(ctx, dto.ProdukCreateInput{
			SKU: sku, Nama: nama, Kategori: kategori,
			SatuanKecilID:  sat.ID,
			FaktorKonversi: 1,
			StokMinimum:    0,
			IsActive:       true,
		}); err != nil {
			res.Failed++
			appendCapped(&res.ErrMsgs, sku+": "+err.Error())
			continue
		}
		res.Imported++
	}
	return res
}

func appendCapped(out *[]string, s string) {
	if len(*out) < 5 {
		*out = append(*out, s)
	}
}

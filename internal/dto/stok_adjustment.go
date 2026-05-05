package dto

import (
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// StokAdjustmentInput payload pembuatan penyesuaian stok (single-step).
type StokAdjustmentInput struct {
	GudangID int64   `form:"gudang_id"`
	ProdukID int64   `form:"produk_id"`
	SatuanID int64   `form:"satuan_id"`
	Qty      float64 `form:"qty"` // bisa negatif
	Kategori string  `form:"kategori"`
	Catatan  string  `form:"catatan"`
}

// Normalize trim whitespace pada string fields.
func (in *StokAdjustmentInput) Normalize() {
	in.Kategori = strings.TrimSpace(in.Kategori)
	in.Catatan = strings.TrimSpace(in.Catatan)
}

// Validate cek invariant input. Tidak pakai validator tag karena rules cross-field.
func (in *StokAdjustmentInput) Validate() error {
	in.Normalize()
	if in.GudangID <= 0 {
		return domain.ErrAdjGudangWajib
	}
	if in.ProdukID <= 0 {
		return domain.ErrAdjProdukWajib
	}
	if in.SatuanID <= 0 {
		return domain.ErrAdjSatuanWajib
	}
	if in.Qty == 0 {
		return domain.ErrAdjQtyNol
	}
	if !domain.IsValidAdjKategori(in.Kategori) {
		return domain.ErrAdjKategoriInvalid
	}
	if len(in.Catatan) > 1024 {
		in.Catatan = in.Catatan[:1024]
	}
	return nil
}

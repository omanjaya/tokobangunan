package dto

import "strings"

// ReturItemInput - 1 baris item form retur penjualan.
type ReturItemInput struct {
	PenjualanItemID int64   `form:"penjualan_item_id"`
	Qty             float64 `form:"qty"`
	SatuanID        int64   `form:"satuan_id"`
}

// ReturPenjualanInput - input membuat retur penjualan baru.
type ReturPenjualanInput struct {
	PenjualanID int64            `form:"penjualan_id"`
	Tanggal     string           `form:"tanggal"`
	Alasan      string           `form:"alasan"`
	Catatan     string           `form:"catatan"`
	Items       []ReturItemInput `form:"-"`
}

// Validate field-level validation, return human-friendly errors.
func (in *ReturPenjualanInput) Validate() FieldErrors {
	errs := FieldErrors{}
	if in.PenjualanID <= 0 {
		errs["penjualan_id"] = "Wajib diisi."
	}
	if strings.TrimSpace(in.Tanggal) == "" {
		errs["tanggal"] = "Wajib diisi."
	}
	if strings.TrimSpace(in.Alasan) == "" {
		errs["alasan"] = "Wajib diisi."
	}
	hasItem := false
	for _, it := range in.Items {
		if it.Qty > 0 {
			hasItem = true
			break
		}
	}
	if !hasItem {
		errs["items"] = "Minimal 1 item dengan qty > 0."
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

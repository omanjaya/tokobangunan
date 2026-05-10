package dto

import (
	"fmt"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// MetodeBreakdownInput satu baris breakdown multi-metode (Rupiah utuh).
type MetodeBreakdownInput struct {
	Metode    string `form:"metode"`
	Jumlah    int64  `form:"jumlah"` // Rupiah utuh
	Referensi string `form:"referensi"`
}

// PembayaranCreateInput input form pencatatan pembayaran customer.
// PenjualanID nil → pembayaran umum (akan dialokasi/disimpan tanpa link).
type PembayaranCreateInput struct {
	PenjualanID *int64 `form:"penjualan_id"`
	MitraID     int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal     string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah      int64  `form:"jumlah" validate:"required,gt=0"` // Rupiah utuh
	Metode      string `form:"metode" validate:"required,oneof=tunai transfer cek giro"`
	Referensi   string `form:"referensi" validate:"max=128"`
	Catatan     string `form:"catatan" validate:"max=512"`
	ClientUUID  string `form:"client_uuid"`
	// MetodeBreakdown opsional. Kalau non-empty, sum(.Jumlah) harus = .Jumlah header.
	MetodeBreakdown []MetodeBreakdownInput
}

// PembayaranBatchInput input form pembayaran batch (FIFO ke invoice tertua).
type PembayaranBatchInput struct {
	MitraID    int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal    string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah     int64  `form:"jumlah" validate:"required,gt=0"`
	Metode     string `form:"metode" validate:"required,oneof=tunai transfer cek giro"`
	Referensi  string `form:"referensi" validate:"max=128"`
	Catatan    string `form:"catatan" validate:"max=512"`
	ClientUUID string `form:"client_uuid"`
}

// ValidateBreakdown extra validation for MetodeBreakdown rows.
// Return field-error map (kosong = OK).
func (in *PembayaranCreateInput) ValidateBreakdown() map[string]string {
	errs := map[string]string{}
	if len(in.MetodeBreakdown) == 0 {
		return errs
	}
	var sum int64
	for i, b := range in.MetodeBreakdown {
		if b.Jumlah <= 0 {
			errs[fmt.Sprintf("metode_breakdown[%d].jumlah", i)] = "jumlah harus > 0"
		}
		if !domain.IsValidMetodeBayar(b.Metode) {
			errs[fmt.Sprintf("metode_breakdown[%d].metode", i)] = "metode tidak valid"
		}
		sum += b.Jumlah
	}
	if sum != in.Jumlah {
		errs["metode_breakdown"] = "total breakdown tidak sama dengan jumlah header"
	}
	return errs
}

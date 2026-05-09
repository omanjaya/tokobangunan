package dto

import (
	"strings"
	"time"
)

// DiskonMasterInput - input form create/update diskon master.
// MinSubtotal & MaxDiskon dalam rupiah utuh (akan dikonversi ke cents di service).
type DiskonMasterInput struct {
	Kode           string  `form:"kode"`
	Nama           string  `form:"nama"`
	Tipe           string  `form:"tipe"`
	Nilai          float64 `form:"nilai"`
	MinSubtotal    int64   `form:"min_subtotal"`
	MaxDiskon      int64   `form:"max_diskon"`
	BerlakuDari    string  `form:"berlaku_dari"`
	BerlakuSampai  string  `form:"berlaku_sampai"`
	IsActive       bool    `form:"is_active"`
}

// Validate cek field-level. Tanggal parse di service.
// Return nil bila valid, FieldErrors bila ada error.
func (in *DiskonMasterInput) Validate() error {
	errs := FieldErrors{}
	if strings.TrimSpace(in.Kode) == "" {
		errs["kode"] = "Wajib diisi."
	}
	if strings.TrimSpace(in.Nama) == "" {
		errs["nama"] = "Wajib diisi."
	}
	if in.Tipe != "persen" && in.Tipe != "nominal" {
		errs["tipe"] = "Pilih persen atau nominal."
	}
	if in.Nilai <= 0 {
		errs["nilai"] = "Harus lebih besar dari 0."
	}
	if in.Tipe == "persen" && in.Nilai > 100 {
		errs["nilai"] = "Persen tidak boleh > 100."
	}
	if in.MinSubtotal < 0 {
		errs["min_subtotal"] = "Tidak boleh negatif."
	}
	if in.MaxDiskon < 0 {
		errs["max_diskon"] = "Tidak boleh negatif."
	}
	if strings.TrimSpace(in.BerlakuDari) == "" {
		errs["berlaku_dari"] = "Wajib diisi."
	} else if _, err := time.Parse("2006-01-02", in.BerlakuDari); err != nil {
		errs["berlaku_dari"] = "Format tanggal harus YYYY-MM-DD."
	}
	if strings.TrimSpace(in.BerlakuSampai) != "" {
		if _, err := time.Parse("2006-01-02", in.BerlakuSampai); err != nil {
			errs["berlaku_sampai"] = "Format tanggal harus YYYY-MM-DD."
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

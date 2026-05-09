package domain

import (
	"errors"
	"time"
)

// Sentinel errors for DiskonMaster.
var (
	ErrDiskonNotFound      = errors.New("diskon tidak ditemukan")
	ErrDiskonKodeWajib     = errors.New("kode diskon wajib diisi")
	ErrDiskonNamaWajib     = errors.New("nama diskon wajib diisi")
	ErrDiskonTipeInvalid   = errors.New("tipe diskon tidak valid")
	ErrDiskonNilaiInvalid  = errors.New("nilai diskon harus > 0")
	ErrDiskonTanggalInvalid = errors.New("tanggal berlaku tidak valid")
	ErrDiskonKodeDuplikat  = errors.New("kode diskon sudah dipakai")
)

// Tipe diskon constants.
const (
	DiskonTipePersen  = "persen"
	DiskonTipeNominal = "nominal"
)

// DiskonMaster - master katalog diskon yang bisa di-apply di POS.
// MinSubtotal & MaxDiskon disimpan dalam cents (rupiah * 100).
type DiskonMaster struct {
	ID             int64
	Kode           string
	Nama           string
	Tipe           string
	Nilai          float64
	MinSubtotal    int64
	MaxDiskon      *int64
	BerlakuDari    time.Time
	BerlakuSampai  *time.Time
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Apply menghitung diskon amount (cents) berdasar subtotal (cents).
// Hasil di-cap MaxDiskon (kalau ada) dan tidak melebihi subtotal.
func (d *DiskonMaster) Apply(subtotal int64) int64 {
	if subtotal < d.MinSubtotal {
		return 0
	}
	var amt int64
	if d.Tipe == DiskonTipePersen {
		amt = int64(float64(subtotal) * d.Nilai / 100)
	} else {
		amt = int64(d.Nilai)
	}
	if d.MaxDiskon != nil && amt > *d.MaxDiskon {
		amt = *d.MaxDiskon
	}
	if amt > subtotal {
		amt = subtotal
	}
	if amt < 0 {
		amt = 0
	}
	return amt
}

// IsApplicable cek apakah diskon valid utk subtotal di waktu t.
func (d *DiskonMaster) IsApplicable(subtotal int64, t time.Time) bool {
	if !d.IsActive {
		return false
	}
	if subtotal < d.MinSubtotal {
		return false
	}
	if t.Before(d.BerlakuDari) {
		return false
	}
	if d.BerlakuSampai != nil && t.After(*d.BerlakuSampai) {
		return false
	}
	return true
}

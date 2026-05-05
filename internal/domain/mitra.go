// Package domain berisi entity bisnis dan invariannya. Tidak boleh import
// repo/service/handler — hanya stdlib + sentinel error.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Sentinel error domain mitra. Handler memetakan ke HTTP status / pesan UX.
var (
	ErrMitraKodeKosong       = errors.New("kode mitra wajib diisi")
	ErrMitraNamaKosong       = errors.New("nama mitra wajib diisi")
	ErrMitraTipeInvalid      = errors.New("tipe mitra harus eceran/grosir/proyek")
	ErrMitraLimitNegatif     = errors.New("limit kredit tidak boleh negatif")
	ErrMitraTempoNegatif     = errors.New("jatuh tempo hari tidak boleh negatif")
	ErrMitraKodeDuplicate    = errors.New("kode mitra sudah dipakai")
	ErrMitraTidakDitemukan   = errors.New("mitra tidak ditemukan")
)

// Tipe mitra yang valid (whitelist).
const (
	MitraTipeEceran = "eceran"
	MitraTipeGrosir = "grosir"
	MitraTipeProyek = "proyek"
)

// Mitra adalah pelanggan toko (eceran, grosir, proyek).
type Mitra struct {
	ID              int64
	Kode            string
	Nama            string
	Alamat          *string
	Kontak          *string
	NPWP            *string
	Tipe            string
	LimitKredit     int64 // cents
	JatuhTempoHari  int
	GudangDefaultID *int64
	Catatan         *string
	IsActive        bool
	Version         int64
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsValidMitraTipe cek apakah tipe ada di whitelist.
func IsValidMitraTipe(tipe string) bool {
	switch tipe {
	case MitraTipeEceran, MitraTipeGrosir, MitraTipeProyek:
		return true
	}
	return false
}

// Validate memeriksa invariant Mitra. Dipanggil sebelum Create/Update.
func (m *Mitra) Validate() error {
	if strings.TrimSpace(m.Kode) == "" {
		return ErrMitraKodeKosong
	}
	if strings.TrimSpace(m.Nama) == "" {
		return ErrMitraNamaKosong
	}
	if !IsValidMitraTipe(m.Tipe) {
		return ErrMitraTipeInvalid
	}
	if m.LimitKredit < 0 {
		return ErrMitraLimitNegatif
	}
	if m.JatuhTempoHari < 0 {
		return ErrMitraTempoNegatif
	}
	return nil
}

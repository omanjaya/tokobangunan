// Package domain berisi entity bisnis dan sentinel error.
// Tidak bergantung pada infrastructure (db, http).
package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrSKUWajib              = errors.New("SKU wajib diisi")
	ErrNamaWajib             = errors.New("nama wajib diisi")
	ErrFaktorKonversiInvalid = errors.New("faktor konversi harus > 0")
	ErrStokMinimumInvalid    = errors.New("stok minimum tidak boleh negatif")
	ErrSatuanKecilWajib      = errors.New("satuan kecil wajib diisi")
	ErrProdukNotFound        = errors.New("produk tidak ditemukan")
	ErrSKUDuplikat           = errors.New("SKU sudah dipakai produk lain")
)

// ErrConflict - sentinel error optimistic concurrency. Digunakan saat row
// sudah dimutasi user lain (version mismatch) sehingga update ditolak.
var ErrConflict = errors.New("data sudah diubah pengguna lain, silakan refresh")

// Produk - master data barang dagangan.
type Produk struct {
	ID             int64
	SKU            string
	Nama           string
	Kategori       *string
	SatuanKecilID  int64
	SatuanBesarID  *int64
	FaktorKonversi float64
	StokMinimum    float64
	FotoURL        *string
	IsActive       bool
	LeadTimeDays   int
	SafetyStock    float64
	Version        int64
	DeletedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate cek invariant entity.
func (p *Produk) Validate() error {
	if strings.TrimSpace(p.SKU) == "" {
		return ErrSKUWajib
	}
	if strings.TrimSpace(p.Nama) == "" {
		return ErrNamaWajib
	}
	if p.SatuanKecilID <= 0 {
		return ErrSatuanKecilWajib
	}
	if p.FaktorKonversi <= 0 {
		return ErrFaktorKonversiInvalid
	}
	if p.StokMinimum < 0 {
		return ErrStokMinimumInvalid
	}
	return nil
}

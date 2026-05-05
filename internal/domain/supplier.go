package domain

import (
	"errors"
	"strings"
	"time"
)

// Sentinel error domain supplier.
var (
	ErrSupplierKodeKosong     = errors.New("kode supplier wajib diisi")
	ErrSupplierNamaKosong     = errors.New("nama supplier wajib diisi")
	ErrSupplierKodeDuplicate  = errors.New("kode supplier sudah dipakai")
	ErrSupplierTidakDitemukan = errors.New("supplier tidak ditemukan")
)

// Supplier adalah vendor / pemasok barang.
type Supplier struct {
	ID        int64
	Kode      string
	Nama      string
	Alamat    *string
	Kontak    *string
	Catatan   *string
	IsActive  bool
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate memeriksa invariant Supplier.
func (s *Supplier) Validate() error {
	if strings.TrimSpace(s.Kode) == "" {
		return ErrSupplierKodeKosong
	}
	if strings.TrimSpace(s.Nama) == "" {
		return ErrSupplierNamaKosong
	}
	return nil
}

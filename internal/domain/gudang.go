package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrGudangKodeWajib    = errors.New("kode gudang wajib diisi")
	ErrGudangKodeFormat   = errors.New("kode gudang harus huruf kapital A-Z atau underscore")
	ErrGudangNamaWajib    = errors.New("nama gudang wajib diisi")
	ErrGudangNotFound     = errors.New("gudang tidak ditemukan")
	ErrGudangKodeDuplikat = errors.New("kode gudang sudah dipakai")
)

// Gudang - master cabang/lokasi.
type Gudang struct {
	ID        int64
	Kode      string
	Nama      string
	Alamat    *string
	Telepon   *string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate cek invariant: Kode wajib uppercase A-Z + underscore, Nama wajib.
func (g *Gudang) Validate() error {
	kode := strings.TrimSpace(g.Kode)
	if kode == "" {
		return ErrGudangKodeWajib
	}
	for _, r := range kode {
		isUpper := r >= 'A' && r <= 'Z'
		if !isUpper && r != '_' {
			return ErrGudangKodeFormat
		}
	}
	if strings.TrimSpace(g.Nama) == "" {
		return ErrGudangNamaWajib
	}
	return nil
}

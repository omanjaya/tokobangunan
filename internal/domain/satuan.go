package domain

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

var (
	ErrSatuanKodeWajib   = errors.New("kode satuan wajib diisi")
	ErrSatuanKodeFormat  = errors.New("kode satuan harus huruf kecil tanpa spasi")
	ErrSatuanNamaWajib   = errors.New("nama satuan wajib diisi")
	ErrSatuanNotFound    = errors.New("satuan tidak ditemukan")
	ErrSatuanKodeDuplikat = errors.New("kode satuan sudah dipakai")
)

// Satuan - master unit (sak, kg, batang, m, m2, lusin, biji, dll).
type Satuan struct {
	ID        int64
	Kode      string
	Nama      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate cek kode lowercase non-empty + nama non-empty.
func (s *Satuan) Validate() error {
	kode := strings.TrimSpace(s.Kode)
	if kode == "" {
		return ErrSatuanKodeWajib
	}
	for _, r := range kode {
		if unicode.IsSpace(r) || (unicode.IsLetter(r) && !unicode.IsLower(r)) {
			return ErrSatuanKodeFormat
		}
	}
	if strings.TrimSpace(s.Nama) == "" {
		return ErrSatuanNamaWajib
	}
	return nil
}

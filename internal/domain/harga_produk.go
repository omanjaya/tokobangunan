package domain

import (
	"errors"
	"time"
)

// Tipe harga.
const (
	TipeHargaEceran = "eceran"
	TipeHargaGrosir = "grosir"
	TipeHargaProyek = "proyek"
)

var (
	ErrHargaProdukWajib   = errors.New("produk wajib diisi")
	ErrHargaTipeInvalid   = errors.New("tipe harga harus eceran/grosir/proyek")
	ErrHargaJualInvalid   = errors.New("harga jual harus > 0")
	ErrHargaTanggalInvalid = errors.New("tanggal berlaku tidak valid")
	ErrHargaNotFound      = errors.New("harga produk tidak ditemukan")
)

// HargaProduk - history harga jual per produk per gudang per tipe.
// HargaJual disimpan dalam cents (BIGINT).
type HargaProduk struct {
	ID          int64
	ProdukID    int64
	GudangID    *int64
	Tipe        string
	HargaJual   int64
	BerlakuDari time.Time
	CreatedAt   time.Time
}

// Validate cek invariant.
func (h *HargaProduk) Validate() error {
	if h.ProdukID <= 0 {
		return ErrHargaProdukWajib
	}
	switch h.Tipe {
	case TipeHargaEceran, TipeHargaGrosir, TipeHargaProyek:
	default:
		return ErrHargaTipeInvalid
	}
	if h.HargaJual <= 0 {
		return ErrHargaJualInvalid
	}
	if h.BerlakuDari.IsZero() {
		return ErrHargaTanggalInvalid
	}
	return nil
}

// IsValidTipeHarga true bila string adalah tipe harga yang dikenali.
func IsValidTipeHarga(t string) bool {
	switch t {
	case TipeHargaEceran, TipeHargaGrosir, TipeHargaProyek:
		return true
	}
	return false
}

package domain

import (
	"errors"
	"time"
)

// Kategori penyesuaian stok (single-step adjustment).
const (
	AdjKategoriInitial        = "initial"
	AdjKategoriKoreksi        = "koreksi"
	AdjKategoriRusak          = "rusak"
	AdjKategoriHilang         = "hilang"
	AdjKategoriSample         = "sample"
	AdjKategoriHadiah         = "hadiah"
	AdjKategoriReturnSupplier = "return_supplier"
	AdjKategoriReturnCustomer = "return_customer"
)

// Sentinel errors stok_adjustment.
var (
	ErrAdjKategoriInvalid    = errors.New("kategori penyesuaian tidak valid")
	ErrAdjQtyNol             = errors.New("qty penyesuaian tidak boleh 0")
	ErrAdjGudangWajib        = errors.New("gudang wajib diisi")
	ErrAdjProdukWajib        = errors.New("produk wajib diisi")
	ErrAdjSatuanWajib        = errors.New("satuan wajib diisi")
	ErrAdjSatuanTidakCocok   = errors.New("satuan tidak cocok dgn produk")
	ErrAdjStokTidakCukup     = errors.New("stok tidak cukup utk penyesuaian negatif")
	ErrAdjTidakDitemukan     = errors.New("penyesuaian stok tidak ditemukan")
)

// StokAdjustment baris audit penyesuaian stok.
type StokAdjustment struct {
	ID          int64
	GudangID    int64
	ProdukID    int64
	SatuanID    int64
	Qty         float64 // input sebelum konversi (signed)
	QtyKonversi float64 // setelah konversi ke satuan kecil (signed)
	Kategori    string
	Alasan      string  // header reason (derived dari kategori)
	Catatan     *string // detail optional
	UserID      int64
	CreatedAt   time.Time

	// Display-only (di-load via JOIN), opsional.
	GudangNama string
	ProdukNama string
	SatuanKode string
	UserNama   string
}

// IsValidAdjKategori cek kategori termasuk salah satu konstan.
func IsValidAdjKategori(k string) bool {
	switch k {
	case AdjKategoriInitial, AdjKategoriKoreksi, AdjKategoriRusak,
		AdjKategoriHilang, AdjKategoriSample, AdjKategoriHadiah,
		AdjKategoriReturnSupplier, AdjKategoriReturnCustomer:
		return true
	}
	return false
}

// AdjAlasanDefault mapping kategori → alasan ringkas (header reason).
func AdjAlasanDefault(kategori string) string {
	switch kategori {
	case AdjKategoriInitial:
		return "Stok awal sistem"
	case AdjKategoriKoreksi:
		return "Koreksi stok"
	case AdjKategoriRusak:
		return "Barang rusak"
	case AdjKategoriHilang:
		return "Barang hilang"
	case AdjKategoriSample:
		return "Sample / contoh"
	case AdjKategoriHadiah:
		return "Hadiah / promo"
	case AdjKategoriReturnSupplier:
		return "Retur ke supplier"
	case AdjKategoriReturnCustomer:
		return "Retur dari customer"
	}
	return "Penyesuaian stok"
}

// AdjKategoriSignHint -1=biasanya negatif, +1=biasanya positif, 0=bebas.
// Service tidak meng-enforce ini; hanya hint utk UI.
func AdjKategoriSignHint(kategori string) int {
	switch kategori {
	case AdjKategoriInitial, AdjKategoriReturnCustomer:
		return +1
	case AdjKategoriRusak, AdjKategoriHilang, AdjKategoriSample,
		AdjKategoriHadiah, AdjKategoriReturnSupplier:
		return -1
	}
	return 0
}

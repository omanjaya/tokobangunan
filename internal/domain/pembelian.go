package domain

import (
	"errors"
	"strings"
	"time"
)

// Sentinel error pembelian.
var (
	ErrPembelianTidakDitemukan   = errors.New("pembelian tidak ditemukan")
	ErrPembelianItemKosong       = errors.New("pembelian harus memiliki minimal 1 item")
	ErrPembelianSupplierWajib    = errors.New("supplier wajib diisi")
	ErrPembelianGudangWajib      = errors.New("gudang wajib diisi")
	ErrPembelianStatusInvalid    = errors.New("status bayar pembelian tidak valid")
	ErrPembelianTotalTidakCocok  = errors.New("total pembelian tidak sesuai subtotal-diskon")
	ErrPembelianCancelBelum      = errors.New("pembelian belum mendukung pembatalan")
	ErrPembayaranSupplierInvalid = errors.New("pembayaran supplier tidak valid")
	ErrMetodeBayarInvalid        = errors.New("metode pembayaran tidak valid")
	ErrPembelianLocked           = errors.New("pembelian tidak bisa diubah karena sudah ada pembayaran")
	ErrPembelianDibatalkan       = errors.New("pembelian sudah dibatalkan")
)

// StatusBayarPembelian status pembayaran pembelian (hutang ke supplier).
type StatusBayarPembelian string

const (
	StatusBeliLunas        StatusBayarPembelian = "lunas"
	StatusBeliKredit       StatusBayarPembelian = "kredit"
	StatusBeliSebagian     StatusBayarPembelian = "sebagian"
	StatusBeliDibatalkan   StatusBayarPembelian = "dibatalkan"
)

// IsValid cek nilai status bayar pembelian.
func (s StatusBayarPembelian) IsValid() bool {
	switch s {
	case StatusBeliLunas, StatusBeliKredit, StatusBeliSebagian, StatusBeliDibatalkan:
		return true
	}
	return false
}

// Pembelian transaksi pembelian dari supplier ke gudang.
type Pembelian struct {
	ID             int64
	NomorPembelian string
	Tanggal        time.Time
	SupplierID     int64
	SupplierNama   string // hydrate-only
	GudangID       int64
	GudangNama     string // hydrate-only
	UserID         int64
	UserNama       string // hydrate-only
	Items          []PembelianItem
	Subtotal       int64
	Diskon         int64
	DPP            int64
	PPNPersen      float64
	PPNAmount      int64
	Total          int64
	StatusBayar    StatusBayarPembelian
	JatuhTempo     *time.Time
	Catatan        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CanceledAt     *time.Time
	CanceledBy     *int64
	CancelReason   *string
}

// PembelianItem baris item pembelian.
type PembelianItem struct {
	ID          int64
	PembelianID int64
	ProdukID    int64
	ProdukNama  string
	Qty         float64
	SatuanID    int64
	SatuanKode  string
	QtyKonversi float64
	HargaSatuan int64
	Subtotal    int64
}

// Validate cek invariant Pembelian.
func (p *Pembelian) Validate() error {
	if p.SupplierID <= 0 {
		return ErrPembelianSupplierWajib
	}
	if p.GudangID <= 0 {
		return ErrPembelianGudangWajib
	}
	if len(p.Items) == 0 {
		return ErrPembelianItemKosong
	}
	if !p.StatusBayar.IsValid() {
		return ErrPembelianStatusInvalid
	}
	expected := p.DPP + p.PPNAmount
	// Backward-compat: kalau DPP belum dihitung (legacy), fallback ke subtotal-diskon.
	if p.DPP == 0 && p.PPNAmount == 0 {
		expected = p.Subtotal - p.Diskon
	}
	if p.Total != expected {
		return ErrPembelianTotalTidakCocok
	}
	return nil
}

// PembayaranSupplier pencatatan pembayaran ke supplier.
type PembayaranSupplier struct {
	ID          int64
	PembelianID *int64
	SupplierID  int64
	Tanggal     time.Time
	Jumlah      int64
	Metode      string // tunai, transfer, cek, giro
	Referensi   string
	UserID      int64
	Catatan     string
	CreatedAt   time.Time
}

// MetodeBayarValid daftar metode yang diterima.
var MetodeBayarValid = map[string]struct{}{
	"tunai":    {},
	"transfer": {},
	"cek":      {},
	"giro":     {},
}

// Validate cek invariant pembayaran supplier.
func (p *PembayaranSupplier) Validate() error {
	if p.SupplierID <= 0 {
		return ErrPembayaranSupplierInvalid
	}
	if p.Jumlah <= 0 {
		return ErrPembayaranSupplierInvalid
	}
	if _, ok := MetodeBayarValid[strings.ToLower(strings.TrimSpace(p.Metode))]; !ok {
		return ErrMetodeBayarInvalid
	}
	return nil
}

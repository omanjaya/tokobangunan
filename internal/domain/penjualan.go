package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// StatusBayar - whitelist status pembayaran penjualan.
type StatusBayar string

const (
	StatusLunas    StatusBayar = "lunas"
	StatusKredit   StatusBayar = "kredit"
	StatusSebagian StatusBayar = "sebagian"
)

// IsValidStatusBayar cek apakah string masuk whitelist.
func IsValidStatusBayar(s string) bool {
	switch StatusBayar(s) {
	case StatusLunas, StatusKredit, StatusSebagian:
		return true
	}
	return false
}

// Sentinel error modul penjualan.
var (
	ErrPenjualanKosong       = errors.New("penjualan harus memiliki minimal 1 item")
	ErrTotalTidakCocok       = errors.New("total tidak cocok dengan subtotal - diskon")
	ErrStatusBayarInvalid    = errors.New("status bayar harus lunas/kredit/sebagian")
	ErrPenjualanNotFound     = errors.New("penjualan tidak ditemukan")
	ErrLimitKreditTerlampaui = errors.New("limit kredit mitra terlampaui")
	ErrJatuhTempoWajib       = errors.New("jatuh tempo wajib diisi untuk kredit/sebagian")
	ErrItemQtyInvalid        = errors.New("qty item harus > 0")
	ErrItemHargaInvalid      = errors.New("harga satuan tidak valid")
)

// Penjualan - header transaksi penjualan.
type Penjualan struct {
	ID            int64
	NomorKwitansi string
	Tanggal       time.Time
	MitraID       int64
	GudangID      int64
	UserID        int64
	Items         []PenjualanItem

	Subtotal    int64 // cents (sebelum diskon header & PPN)
	Diskon      int64 // cents (diskon header)
	DPP         int64 // cents (Dasar Pengenaan Pajak = Subtotal - Diskon)
	PPNPersen   float64
	PPNAmount   int64 // cents
	Total       int64 // cents (DPP + PPNAmount)
	StatusBayar StatusBayar
	JatuhTempo  *time.Time
	Catatan     string

	ClientUUID uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PenjualanItem - baris item, dengan snapshot nama produk & kode satuan.
type PenjualanItem struct {
	ID          int64
	ProdukID    int64
	ProdukNama  string
	Qty         float64
	SatuanID    int64
	SatuanKode  string
	QtyKonversi float64 // qty dalam satuan kecil
	HargaSatuan int64   // cents per satuan
	Diskon      int64   // cents diskon per baris (potong setelah qty*harga)
	Subtotal    int64   // cents - sudah dikurangi Diskon
}

// HitungTotal recompute Subtotal & Total dari Items + Diskon.
// Dipanggil sebelum persist agar konsisten.
func (p *Penjualan) HitungTotal() {
	var sub int64
	for i := range p.Items {
		// Pastikan subtotal item juga konsisten.
		hargaCents := p.Items[i].HargaSatuan
		qty := p.Items[i].Qty
		// Subtotal item = harga * qty (dengan rounding ke int).
		// qty bisa pecahan; kalikan via float lalu bulatkan.
		raw := int64(qty*float64(hargaCents) + 0.5)
		if p.Items[i].Diskon < 0 {
			p.Items[i].Diskon = 0
		}
		s := raw - p.Items[i].Diskon
		if s < 0 {
			s = 0
		}
		p.Items[i].Subtotal = s
		sub += s
	}
	p.Subtotal = sub
	if p.Diskon < 0 {
		p.Diskon = 0
	}
	dpp := sub - p.Diskon
	if dpp < 0 {
		dpp = 0
	}
	p.DPP = dpp
	if p.PPNPersen > 0 {
		p.PPNAmount = int64(float64(dpp)*p.PPNPersen/100 + 0.5)
	} else {
		p.PPNAmount = 0
	}
	p.Total = p.DPP + p.PPNAmount
}

// Validate cek invariant entity Penjualan.
// HitungTotal sebaiknya sudah dipanggil sebelum Validate.
func (p *Penjualan) Validate() error {
	if len(p.Items) == 0 {
		return ErrPenjualanKosong
	}
	for i := range p.Items {
		it := &p.Items[i]
		if it.Qty <= 0 {
			return ErrItemQtyInvalid
		}
		if it.HargaSatuan < 0 {
			return ErrItemHargaInvalid
		}
	}
	if !IsValidStatusBayar(string(p.StatusBayar)) {
		return ErrStatusBayarInvalid
	}
	if (p.StatusBayar == StatusKredit || p.StatusBayar == StatusSebagian) && p.JatuhTempo == nil {
		return ErrJatuhTempoWajib
	}
	expected := p.DPP + p.PPNAmount
	// Backward-compat: kalau DPP belum dihitung, fallback ke Subtotal-Diskon.
	if p.DPP == 0 && p.PPNAmount == 0 {
		expected = p.Subtotal - p.Diskon
	}
	if p.Total != expected {
		return ErrTotalTidakCocok
	}
	return nil
}

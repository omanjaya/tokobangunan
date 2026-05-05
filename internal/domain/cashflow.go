package domain

import (
	"errors"
	"time"
)

// CashflowTipe - "masuk" / "keluar".
type CashflowTipe string

const (
	CashflowMasuk  CashflowTipe = "masuk"
	CashflowKeluar CashflowTipe = "keluar"
)

// IsValid cek nilai tipe cashflow.
func (t CashflowTipe) IsValid() bool {
	return t == CashflowMasuk || t == CashflowKeluar
}

// Sentinel error.
var (
	ErrCashflowNotFound      = errors.New("cashflow tidak ditemukan")
	ErrCashflowJumlahInvalid = errors.New("jumlah cashflow harus > 0")
	ErrCashflowTipeInvalid   = errors.New("tipe cashflow tidak valid")
	ErrCashflowKategoriWajib = errors.New("kategori wajib diisi")
)

// Cashflow - 1 transaksi kas masuk/keluar non-penjualan/pembelian.
type Cashflow struct {
	ID        int64
	Nomor     string
	Tanggal   time.Time
	GudangID  *int64
	Tipe      CashflowTipe
	Kategori  string
	Deskripsi string
	Jumlah    int64 // cents
	Metode    string
	Referensi string
	UserID    int64
	Catatan   string
	CreatedAt time.Time
}

// CashflowKategori - master list kategori kas.
type CashflowKategori struct {
	ID   int64
	Nama string
	Tipe CashflowTipe
}

// Validate cek invariant Cashflow.
func (c *Cashflow) Validate() error {
	if !c.Tipe.IsValid() {
		return ErrCashflowTipeInvalid
	}
	if c.Jumlah <= 0 {
		return ErrCashflowJumlahInvalid
	}
	if c.Kategori == "" {
		return ErrCashflowKategoriWajib
	}
	return nil
}

// CashflowSummary - rekap periode.
type CashflowSummary struct {
	TotalMasuk  int64
	TotalKeluar int64
	NetCashflow int64
}

// CashflowKategoriBreakdown - top kategori utk laporan.
type CashflowKategoriBreakdown struct {
	Kategori string
	Total    int64
	Count    int
}

// CashflowDailyPoint - 1 titik tren harian.
type CashflowDailyPoint struct {
	Tanggal time.Time
	Masuk   int64
	Keluar  int64
}

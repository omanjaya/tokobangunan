// Package stok berisi view templ untuk modul stok per cabang.
package stok

import (
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// GudangLite untuk tab/dropdown.
type GudangLite struct {
	ID   int64
	Kode string
	Nama string
}

// IndexProps - landing page stok dengan tab gudang.
type IndexProps struct {
	Nav            layout.NavData
	User           layout.UserData
	Gudangs        []GudangLite
	ActiveGudangID int64
	Total          int
	Page           int
	PerPage        int
	TotalPages     int
	Rows           []repo.StokDetail
	Query          string
	Kategori       string
	LowStockOnly   bool
	Kategoris      []string
	LowStockCount  int
}

// ProdukDetailProps - posisi stok 1 produk di semua gudang.
type ProdukDetailProps struct {
	Nav        layout.NavData
	User       layout.UserData
	ProdukID   int64
	ProdukNama string
	ProdukSKU  string
	Rows       []repo.StokDetail
}

// StokStatusVariant - badge variant berdasar qty vs stok_minimum.
func StokStatusVariant(qty, minimum float64) string {
	if qty <= 0 {
		return "danger"
	}
	if qty <= minimum {
		return "warning"
	}
	return "success"
}

// StokStatusLabel - label berdasar qty vs minimum.
func StokStatusLabel(qty, minimum float64) string {
	if qty <= 0 {
		return "Habis"
	}
	if qty <= minimum {
		return "Kritis"
	}
	return "Aman"
}

// AdjustFormProps - props halaman form Penyesuaian Stok.
type AdjustFormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	Input   dto.StokAdjustmentInput
	Errors  dto.FieldErrors
	General string
	Success string
	Gudangs []GudangLite
}

// AdjustHistoryRow - 1 baris riwayat penyesuaian.
type AdjustHistoryRow struct {
	ID         int64
	Tanggal    time.Time
	GudangNama string
	ProdukNama string
	ProdukSKU  string
	Qty        float64
	SatuanKode string
	Kategori   string
	Catatan    string
	UserNama   string
}

// AdjustHistoryProps - props halaman riwayat.
type AdjustHistoryProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []AdjustHistoryRow
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	From       string
	To         string
	GudangID   int64
	Kategori   string
	Gudangs    []GudangLite
}

// KategoriOption - opsi dropdown kategori adjustment.
type KategoriOption struct {
	Value string
	Label string
}

// AdjKategoriOptions - daftar opsi kategori untuk dropdown form.
func AdjKategoriOptions() []KategoriOption {
	return []KategoriOption{
		{domain.AdjKategoriInitial, "Stok Awal (Setup Sistem)"},
		{domain.AdjKategoriKoreksi, "Koreksi Input"},
		{domain.AdjKategoriRusak, "Barang Rusak"},
		{domain.AdjKategoriHilang, "Hilang / Selisih Opname"},
		{domain.AdjKategoriSample, "Sample / Marketing"},
		{domain.AdjKategoriHadiah, "Hadiah / Promo"},
		{domain.AdjKategoriReturnSupplier, "Return ke Supplier"},
		{domain.AdjKategoriReturnCustomer, "Return dari Customer"},
	}
}

// AdjKategoriLabel - label readable dari kode kategori.
func AdjKategoriLabel(k string) string {
	for _, o := range AdjKategoriOptions() {
		if o.Value == k {
			return o.Label
		}
	}
	return k
}

// AdjKategoriBadgeVariant - badge variant per kategori.
func AdjKategoriBadgeVariant(k string) string {
	switch k {
	case domain.AdjKategoriInitial, domain.AdjKategoriKoreksi:
		return "info"
	case domain.AdjKategoriRusak, domain.AdjKategoriHilang:
		return "danger"
	case domain.AdjKategoriSample, domain.AdjKategoriHadiah:
		return "warning"
	case domain.AdjKategoriReturnSupplier, domain.AdjKategoriReturnCustomer:
		return "success"
	default:
		return "neutral"
	}
}


// Package stok berisi view templ untuk modul stok per cabang.
package stok

import (
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

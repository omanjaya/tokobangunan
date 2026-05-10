// Package laporan berisi view templates untuk modul laporan & dashboard analitik.
package laporan

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// GudangLite - representasi ringan untuk dropdown filter.
type GudangLite struct {
	ID   int64
	Kode string
	Nama string
}

// IndexProps - landing laporan.
type IndexProps struct {
	Nav  layout.NavData
	User layout.UserData
}

// LRProps - laporan laba-rugi per gudang.
type LRProps struct {
	Nav  layout.NavData
	User layout.UserData
	From string
	To   string
	Rows []repo.LaporanLR
}

// PenjualanProps - laporan penjualan list + filter + pagination.
type PenjualanProps struct {
	Nav        layout.NavData
	User       layout.UserData
	From       string
	To         string
	GudangID   int64
	MitraID    int64
	Rows       []repo.LaporanPenjualanRow
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	Gudangs    []GudangLite
}

// MutasiProps - laporan mutasi periode.
type MutasiProps struct {
	Nav      layout.NavData
	User     layout.UserData
	From     string
	To       string
	GudangID int64
	Rows     []repo.LaporanMutasiRow
	Gudangs  []GudangLite
}

// StokKritisProps.
type StokKritisProps struct {
	Nav  layout.NavData
	User layout.UserData
	Rows []repo.StokKritisRow
}

// TopProdukProps.
type TopProdukProps struct {
	Nav  layout.NavData
	User layout.UserData
	From string
	To   string
	Rows []repo.TopProduk
}

// ReorderProps - laporan inventory forecasting / reorder point.
type ReorderProps struct {
	Nav          layout.NavData
	User         layout.UserData
	GudangID     int64
	LookbackDays int
	Rows         []repo.ProdukVelocity
	Gudangs      []GudangLite
}

// CashflowProps - laporan cashflow periode.
type CashflowProps struct {
	Nav      layout.NavData
	User     layout.UserData
	From     string
	To       string
	GudangID int64
	Gudangs  []GudangLite

	Summary    domain.CashflowSummary
	PrevDelta  int64 // selisih net vs periode sebelumnya
	Items      []domain.Cashflow
	TopKategori []domain.CashflowKategoriBreakdown
	Daily      []domain.CashflowDailyPoint
}

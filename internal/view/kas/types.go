// Package kas berisi view templates untuk modul cashflow (kas masuk/keluar).
package kas

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// GudangLite - dropdown.
type GudangLite struct {
	ID   int64
	Kode string
	Nama string
}

// IndexProps - list cashflow + summary.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.Cashflow
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	From     string
	To       string
	GudangID int64
	Tipe     string
	Kategori string

	Summary domain.CashflowSummary
	Gudangs []GudangLite
}

// FormProps - input cashflow baru.
type FormProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Input      dto.CashflowCreateInput
	General    string
	Errors     dto.FieldErrors
	Gudangs    []GudangLite
	KategoriIn []domain.CashflowKategori
	KategoriOut []domain.CashflowKategori
}

// ShowProps - detail.
type ShowProps struct {
	Nav      layout.NavData
	User     layout.UserData
	Cashflow *domain.Cashflow
	Gudang   *domain.Gudang
	IsOwner  bool
}

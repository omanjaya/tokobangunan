// Package retur_penjualan berisi templ view modul retur penjualan.
package retur_penjualan

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - props halaman list retur.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Rows       []repo.ReturWithRelations
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	From, To   string
}

// FormProps - props halaman form retur (prefilled dari invoice).
type FormProps struct {
	Nav       layout.NavData
	User      layout.UserData
	Penjualan *domain.Penjualan
	MitraNama string
	Tanggal   string
	General   string
	CSRFToken string
}

// ShowProps - props halaman detail retur.
type ShowProps struct {
	Nav       layout.NavData
	User      layout.UserData
	Retur     *domain.ReturPenjualan
	Penjualan *domain.Penjualan
	MitraNama string
}

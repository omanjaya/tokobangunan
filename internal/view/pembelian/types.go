// Package pembelian berisi templ view untuk modul Pembelian (hutang supplier)
// + form pembayaran ke supplier.
package pembelian

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - data halaman list pembelian.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.Pembelian
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	// Filter
	SupplierID int64
	GudangID   int64
	Status     string
	From       string
	To         string

	Gudangs   []domain.Gudang
	Suppliers []domain.Supplier

	FlashSuccess string
}

// FormProps - data halaman create pembelian.
type FormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	Input   dto.PembelianCreateInput
	Errors  map[string]string
	General string

	Gudangs   []domain.Gudang
	Suppliers []domain.Supplier
	Satuans   []domain.Satuan
}

// ShowProps - data halaman detail pembelian.
type ShowProps struct {
	Nav         layout.NavData
	User        layout.UserData
	Pembelian   *domain.Pembelian
	Pembayarans []domain.PembayaranSupplier
	Sisa        int64
	GeneralErr  string
}

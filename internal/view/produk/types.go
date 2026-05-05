// Package produk berisi view templ untuk modul produk.
package produk

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// HargaSnapshot - harga eceran terkini (nullable) untuk row tabel.
type HargaSnapshot struct {
	Cents int64
	HasValue bool
}

// SatuanLite ringkas untuk dropdown.
type SatuanLite struct {
	ID   int64
	Kode string
	Nama string
}

// Row gabungan produk + display fields.
type Row struct {
	Produk      domain.Produk
	SatuanKecil string
	SatuanBesar string // empty bila nil
	HargaEceran HargaSnapshot
}

// IndexProps - props halaman list produk.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Rows       []Row
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	Query      string
	Kategori   string
	OnlyActive bool
	Kategoris  []string
}

// FormProps - props halaman form create/edit.
type FormProps struct {
	Nav      layout.NavData
	User     layout.UserData
	IsEdit   bool
	ID       int64
	Input    dto.ProdukUpdateInput // dipakai untuk repopulate form
	Errors   dto.FieldErrors
	General  string // error umum (mis. SKU duplikat)
	Satuans  []SatuanLite
	FotoURL  *string // URL foto produk (kalau ada)
}

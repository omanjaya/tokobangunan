// Package opname berisi templ view untuk modul Stok Opname.
package opname

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - props halaman list opname.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Items      []domain.StokOpname
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	GudangID int64
	Status   string

	Gudangs []domain.Gudang
}

// FormProps - props halaman create opname (header only).
type FormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	Input   dto.StokOpnameCreateInput
	Errors  map[string]string
	General string

	Gudangs []domain.Gudang
}

// ShowProps - props halaman detail opname (item editable bila draft).
type ShowProps struct {
	Nav    layout.NavData
	User   layout.UserData
	Opname *domain.StokOpname
}

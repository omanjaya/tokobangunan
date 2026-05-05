// Package satuan berisi view templ untuk modul master satuan.
package satuan

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// IndexProps - props halaman list satuan.
type IndexProps struct {
	Nav     layout.NavData
	User    layout.UserData
	Items   []domain.Satuan
	Input   dto.SatuanCreateInput
	Errors  dto.FieldErrors
	General string
	Success string
}

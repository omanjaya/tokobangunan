// Package tabungan berisi templ view untuk tabungan mitra (saldo titip).
package tabungan

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// ShowProps - halaman saldo + history + form setor/tarik.
type ShowProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Mitra      *domain.Mitra
	Saldo      int64
	Items      []domain.TabunganMitra
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	From string
	To   string

	FlashMsg string
	FlashErr string

	CSRFToken string
}

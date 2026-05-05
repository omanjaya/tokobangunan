// Package pembayaran berisi templ view untuk history pembayaran customer
// (mitra). Form pembayaran utama tinggal di view/piutang/mitra_detail.
package pembayaran

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// HistoryProps history pembayaran 1 mitra.
type HistoryProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Mitra      *domain.Mitra
	Items      []domain.Pembayaran
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	From string
	To   string
}

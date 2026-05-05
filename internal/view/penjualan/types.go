// Package penjualan berisi view templ untuk modul penjualan + kwitansi.
package penjualan

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// MitraLite ringkas untuk dropdown / autocomplete.
type MitraLite struct {
	ID    int64
	Kode  string
	Nama  string
	Tipe  string
	Limit int64 // cents
}

// GudangLite ringkas untuk dropdown.
type GudangLite struct {
	ID   int64
	Kode string
	Nama string
}

// Row - 1 baris di tabel index.
type Row struct {
	Penjualan domain.Penjualan
	MitraNama string
	GudangKode string
}

// IndexProps - props halaman list.
type IndexProps struct {
	Nav        layout.NavData
	User       layout.UserData
	Rows       []Row
	Total      int
	Page       int
	PerPage    int
	TotalPages int

	// Filter
	From       string
	To         string
	GudangID   int64
	MitraID    int64
	Status     string
	Query      string

	Gudangs []GudangLite
}

// FormProps - props halaman form penjualan baru.
type FormProps struct {
	Nav     layout.NavData
	User    layout.UserData
	Input   dto.PenjualanCreateInput
	Errors  dto.FieldErrors
	General string

	Gudangs    []GudangLite
	MitraNama  string // bila MitraID terisi (echo back)
	ClientUUID string // di-set server, dipakai sebagai hidden field

	// PPN config dari app_setting; nil-safe.
	PPNAvailable bool    // true kalau pajak_config.ppn_enabled
	PPNPersen    float64 // persentase aktif (mis. 11.0)
}

// ShowProps - props halaman detail.
type ShowProps struct {
	Nav       layout.NavData
	User      layout.UserData
	Penjualan *domain.Penjualan
	Mitra     *domain.Mitra
	Gudang    *domain.Gudang
}

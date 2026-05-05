// Package mutasi berisi view templ untuk modul mutasi antar gudang.
package mutasi

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// GudangLite ringkas untuk dropdown.
type GudangLite struct {
	ID   int64
	Kode string
	Nama string
}

// SatuanLite ringkas untuk dropdown item form.
type SatuanLite struct {
	ID   int64
	Kode string
	Nama string
}

// Row gabungan mutasi + nama gudang untuk display di list.
type Row struct {
	Mutasi          domain.MutasiGudang
	GudangAsalNama  string
	GudangTujuanNama string
	JumlahItem      int
}

// IndexProps - props halaman list mutasi.
type IndexProps struct {
	Nav            layout.NavData
	User           layout.UserData
	Rows           []Row
	Total          int
	Page           int
	PerPage        int
	TotalPages     int
	From           string
	To             string
	GudangAsalID   int64
	GudangTujuanID int64
	Status         string
	Gudangs        []GudangLite
}

// FormProps - props halaman form create.
type FormProps struct {
	Nav         layout.NavData
	User        layout.UserData
	Input       dto.MutasiCreateInput
	Errors      dto.FieldErrors
	General     string
	Gudangs     []GudangLite
	Satuans     []SatuanLite
	DefaultDate string
	ClientUUID  string
}

// ShowItem display row item dengan field harga formatted.
type ShowItem struct {
	Item              domain.MutasiItem
	HargaInternalText string
}

// ShowProps - props halaman detail.
type ShowProps struct {
	Nav              layout.NavData
	User             layout.UserData
	Mutasi           domain.MutasiGudang
	GudangAsalNama   string
	GudangTujuanNama string
	UserPengirimName string
	UserPenerimaName string
	Items            []ShowItem
	CanSubmit        bool
	CanReceive       bool
	CanCancel        bool
	FlashError       string
}

// StatusBadgeVariant - mapping status -> badge variant.
func StatusBadgeVariant(s domain.StatusMutasi) string {
	switch s {
	case domain.StatusDraft:
		return "default"
	case domain.StatusDikirim:
		return "info"
	case domain.StatusDiterima:
		return "success"
	case domain.StatusDibatalkan:
		return "default"
	}
	return "default"
}

// StatusLabel - label human-friendly.
func StatusLabel(s domain.StatusMutasi) string {
	switch s {
	case domain.StatusDraft:
		return "Draft"
	case domain.StatusDikirim:
		return "Dikirim"
	case domain.StatusDiterima:
		return "Diterima"
	case domain.StatusDibatalkan:
		return "Dibatalkan"
	}
	return string(s)
}

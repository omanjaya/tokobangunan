package domain

import "time"

// PiutangAging - bucket aging piutang berdasarkan hari overdue.
type PiutangAging string

const (
	AgingCurrent PiutangAging = "current" // belum jatuh tempo
	Aging1to30   PiutangAging = "1-30"
	Aging31to60  PiutangAging = "31-60"
	Aging61to90  PiutangAging = "61-90"
	Aging90Plus  PiutangAging = "90+"
)

// IsValidAging cek whitelist.
func IsValidAging(s string) bool {
	switch PiutangAging(s) {
	case AgingCurrent, Aging1to30, Aging31to60, Aging61to90, Aging90Plus:
		return true
	}
	return false
}

// AgingFromDays mapping hari overdue ke bucket.
// hari < 0  → Current; 0..30 → 1-30; 31..60 → 31-60; 61..90 → 61-90; >90 → 90+.
func AgingFromDays(hari int) PiutangAging {
	switch {
	case hari <= 0:
		return AgingCurrent
	case hari <= 30:
		return Aging1to30
	case hari <= 60:
		return Aging31to60
	case hari <= 90:
		return Aging61to90
	default:
		return Aging90Plus
	}
}

// PiutangSummary ringkasan piutang per mitra (untuk index).
type PiutangSummary struct {
	MitraID        int64
	MitraNama      string
	MitraKode      string
	TotalPenjualan int64
	TotalDibayar   int64
	Outstanding    int64
	InvoiceTertua  *time.Time
	Aging          PiutangAging
	JumlahInvoice  int
}

// PiutangInvoice satu baris invoice belum lunas.
type PiutangInvoice struct {
	PenjualanID      int64
	PenjualanTanggal time.Time
	NomorKwitansi    string
	Tanggal          time.Time
	JatuhTempo       *time.Time
	Total            int64
	Dibayar          int64
	Outstanding      int64
	HariOverdue      int // 0 kalau belum overdue
	Aging            PiutangAging
}

package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sentinel error pembayaran customer (mitra).
var (
	ErrPembayaranInvalid          = errors.New("pembayaran tidak valid")
	ErrJumlahLebihDariOutstanding = errors.New("jumlah pembayaran melebihi outstanding")
	ErrPembayaranNotFound         = errors.New("pembayaran tidak ditemukan")
	ErrPembayaranCancelBelum      = errors.New("pembayaran belum mendukung pembatalan")
)

// MetodeBayar - whitelist metode pembayaran.
type MetodeBayar string

const (
	MetodeTunai    MetodeBayar = "tunai"
	MetodeTransfer MetodeBayar = "transfer"
	MetodeCek      MetodeBayar = "cek"
	MetodeGiro     MetodeBayar = "giro"
)

// IsValidMetodeBayar cek whitelist.
func IsValidMetodeBayar(s string) bool {
	switch MetodeBayar(strings.ToLower(strings.TrimSpace(s))) {
	case MetodeTunai, MetodeTransfer, MetodeCek, MetodeGiro:
		return true
	}
	return false
}

// MetodePembayaranBreakdown - satu komponen breakdown multi-metode pada
// pembayaran. Jumlah dalam cents (sama dengan kolom Pembayaran.Jumlah).
type MetodePembayaranBreakdown struct {
	Metode    string `json:"metode"`
	Jumlah    int64  `json:"jumlah"` // cents
	Referensi string `json:"referensi,omitempty"`
}

// Pembayaran - pembayaran customer (mitra) ke penjualan.
// Bila PenjualanID nil → pembayaran umum (belum dialokasikan).
type Pembayaran struct {
	ID               int64
	PenjualanID      *int64
	PenjualanTanggal *time.Time
	MitraID          int64
	Tanggal          time.Time
	Jumlah           int64 // cents
	Metode           MetodeBayar
	Referensi        string
	UserID           int64
	Catatan          string
	ClientUUID       uuid.UUID
	CreatedAt        time.Time
	// MetodeBreakdown - opsional. Kalau non-nil, sum(jumlah) komponen harus
	// sama dengan Jumlah header (DB trigger enforce). NULL/empty = single metode.
	MetodeBreakdown []MetodePembayaranBreakdown
}

// Validate cek invariant.
func (p *Pembayaran) Validate() error {
	if p.MitraID <= 0 {
		return ErrPembayaranInvalid
	}
	if p.Jumlah <= 0 {
		return ErrPembayaranInvalid
	}
	if !IsValidMetodeBayar(string(p.Metode)) {
		return ErrPembayaranInvalid
	}
	if p.Tanggal.IsZero() {
		return ErrPembayaranInvalid
	}
	// Konsistensi: kalau penjualan_id ada, tanggal juga harus ada (FK composite).
	if (p.PenjualanID == nil) != (p.PenjualanTanggal == nil) {
		return ErrPembayaranInvalid
	}
	return nil
}

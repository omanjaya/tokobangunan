package domain

import (
	"errors"
	"time"
)

// Sentinel error modul retur penjualan.
var (
	ErrReturPenjualanKosong   = errors.New("retur harus memiliki minimal 1 item")
	ErrReturPenjualanNotFound = errors.New("retur penjualan tidak ditemukan")
	ErrReturQtyMelebihi       = errors.New("qty retur melebihi qty tersedia di invoice")
	ErrReturInvoiceDibatalkan = errors.New("invoice sudah dibatalkan, tidak bisa diretur")
	ErrReturItemNotFound      = errors.New("item invoice tidak ditemukan")
)

// ReturPenjualan - header retur penjualan.
type ReturPenjualan struct {
	ID               int64
	NomorRetur       string
	PenjualanID      int64
	PenjualanTanggal time.Time
	MitraID          *int64
	GudangID         int64
	Tanggal          time.Time
	Alasan           string
	Catatan          string
	SubtotalRefund   int64 // cents
	UserID           int64
	CreatedAt        time.Time

	Items []ReturPenjualanItem
}

// ReturPenjualanItem - baris retur.
type ReturPenjualanItem struct {
	ID              int64
	ReturID         int64
	PenjualanItemID int64
	ProdukID        int64
	ProdukNama      string // join-time, tidak persisted
	Qty             float64
	QtyKonversi     float64
	SatuanID        int64
	SatuanKode      string // join-time
	HargaSatuan     int64  // cents
	Subtotal        int64  // cents
}

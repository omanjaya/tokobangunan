package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StatusMutasi - status workflow mutasi antar gudang.
type StatusMutasi string

const (
	StatusDraft      StatusMutasi = "draft"
	StatusDikirim    StatusMutasi = "dikirim"
	StatusDiterima   StatusMutasi = "diterima"
	StatusDibatalkan StatusMutasi = "dibatalkan"
)

// IsValid cek apakah status string termasuk enum valid.
func (s StatusMutasi) IsValid() bool {
	switch s {
	case StatusDraft, StatusDikirim, StatusDiterima, StatusDibatalkan:
		return true
	}
	return false
}

// Sentinel error mutasi.
var (
	ErrMutasiKosong        = errors.New("item mutasi tidak boleh kosong")
	ErrAsalSamaTujuan      = errors.New("gudang asal dan tujuan tidak boleh sama")
	ErrTransisiInvalid     = errors.New("transisi status mutasi tidak valid")
	ErrMutasiNotFound      = errors.New("mutasi tidak ditemukan")
	ErrMutasiStatusInvalid = errors.New("status mutasi tidak valid")
	ErrStokTidakCukup      = errors.New("stok di gudang asal tidak mencukupi")
	ErrNomorMutasiKosong   = errors.New("nomor mutasi wajib diisi")
)

// MutasiGudang - header dokumen mutasi antar gudang.
type MutasiGudang struct {
	ID             int64
	NomorMutasi    string
	Tanggal        time.Time
	GudangAsalID   int64
	GudangTujuanID int64
	Status         StatusMutasi
	UserPengirimID *int64
	UserPenerimaID *int64
	TanggalKirim   *time.Time
	TanggalTerima  *time.Time
	Catatan        string
	ClientUUID     uuid.UUID
	Items          []MutasiItem
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MutasiItem - line item mutasi. produk_nama dan satuan_kode di-snapshot.
// QtyKonversi adalah qty dalam satuan_kecil (untuk perhitungan stok).
type MutasiItem struct {
	ID            int64
	MutasiID      int64
	ProdukID      int64
	ProdukNama    string
	Qty           float64
	SatuanID      int64
	SatuanKode    string
	QtyKonversi   float64
	HargaInternal *int64
	Catatan       string
}

// Validate cek invariant entity mutasi.
func (m *MutasiGudang) Validate() error {
	if strings.TrimSpace(m.NomorMutasi) == "" {
		return ErrNomorMutasiKosong
	}
	if m.GudangAsalID <= 0 || m.GudangTujuanID <= 0 {
		return ErrAsalSamaTujuan
	}
	if m.GudangAsalID == m.GudangTujuanID {
		return ErrAsalSamaTujuan
	}
	if !m.Status.IsValid() {
		return ErrMutasiStatusInvalid
	}
	if len(m.Items) == 0 {
		return ErrMutasiKosong
	}
	for i := range m.Items {
		if m.Items[i].Qty <= 0 || m.Items[i].QtyKonversi <= 0 {
			return ErrMutasiKosong
		}
		if m.Items[i].ProdukID <= 0 || m.Items[i].SatuanID <= 0 {
			return ErrMutasiKosong
		}
	}
	return nil
}

// CanTransitionTo cek apakah perpindahan status legal.
// Aturan:
//   - draft -> dikirim, draft -> dibatalkan
//   - dikirim -> diterima, dikirim -> dibatalkan (revert stok ke gudang asal)
func (s StatusMutasi) CanTransitionTo(next StatusMutasi) bool {
	switch s {
	case StatusDraft:
		return next == StatusDikirim || next == StatusDibatalkan
	case StatusDikirim:
		return next == StatusDiterima || next == StatusDibatalkan
	}
	return false
}

package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrPrinterTemplateNamaWajib  = errors.New("nama template wajib diisi")
	ErrPrinterTemplateJenisWajib = errors.New("jenis template wajib diisi")
	ErrPrinterTemplateJenisInval = errors.New("jenis template tidak valid")
	ErrPrinterTemplateGudangReq  = errors.New("gudang wajib dipilih")
	ErrPrinterTemplateNotFound   = errors.New("template printer tidak ditemukan")
	ErrPrinterTemplateDuplikat   = errors.New("nama template sudah dipakai pada gudang & jenis ini")
)

// PrinterJenis valid: kwitansi | struk | label.
var validPrinterJenis = map[string]bool{
	"kwitansi": true,
	"struk":    true,
	"label":    true,
}

// PrinterTemplate - konfigurasi printer dot matrix per gudang.
type PrinterTemplate struct {
	ID           int64
	GudangID     int64
	Jenis        string
	Nama         string
	LebarChar    int
	PanjangBaris int
	OffsetX      int
	OffsetY      int
	Koordinat    string // JSON string mentah
	Preview      *string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validate cek invariant minimal.
func (t *PrinterTemplate) Validate() error {
	if t.GudangID <= 0 {
		return ErrPrinterTemplateGudangReq
	}
	jenis := strings.TrimSpace(t.Jenis)
	if jenis == "" {
		return ErrPrinterTemplateJenisWajib
	}
	if !validPrinterJenis[jenis] {
		return ErrPrinterTemplateJenisInval
	}
	if strings.TrimSpace(t.Nama) == "" {
		return ErrPrinterTemplateNamaWajib
	}
	if t.LebarChar <= 0 {
		t.LebarChar = 80
	}
	if t.PanjangBaris <= 0 {
		t.PanjangBaris = 33
	}
	if strings.TrimSpace(t.Koordinat) == "" {
		t.Koordinat = "{}"
	}
	return nil
}

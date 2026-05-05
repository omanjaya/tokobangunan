package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPembayaran_Validate(t *testing.T) {
	now := time.Now()
	pid := int64(10)
	pdate := now.Add(-24 * time.Hour)

	base := func() *Pembayaran {
		return &Pembayaran{
			MitraID: 1,
			Tanggal: now,
			Jumlah:  10000,
			Metode:  MetodeTunai,
		}
	}

	tests := []struct {
		name    string
		mutate  func(p *Pembayaran)
		wantErr error
	}{
		{"ok tunai", func(p *Pembayaran) {}, nil},
		{"ok transfer", func(p *Pembayaran) { p.Metode = MetodeTransfer }, nil},
		{"mitra id nol", func(p *Pembayaran) { p.MitraID = 0 }, ErrPembayaranInvalid},
		{"mitra id negatif", func(p *Pembayaran) { p.MitraID = -1 }, ErrPembayaranInvalid},
		{"jumlah nol", func(p *Pembayaran) { p.Jumlah = 0 }, ErrPembayaranInvalid},
		{"jumlah negatif", func(p *Pembayaran) { p.Jumlah = -100 }, ErrPembayaranInvalid},
		{"metode kosong", func(p *Pembayaran) { p.Metode = "" }, ErrPembayaranInvalid},
		{"metode invalid", func(p *Pembayaran) { p.Metode = MetodeBayar("kartu") }, ErrPembayaranInvalid},
		{"tanggal kosong", func(p *Pembayaran) { p.Tanggal = time.Time{} }, ErrPembayaranInvalid},
		{
			"penjualan id tanpa tanggal",
			func(p *Pembayaran) { p.PenjualanID = &pid },
			ErrPembayaranInvalid,
		},
		{
			"penjualan tanggal tanpa id",
			func(p *Pembayaran) { p.PenjualanTanggal = &pdate },
			ErrPembayaranInvalid,
		},
		{
			"penjualan id dan tanggal ok",
			func(p *Pembayaran) {
				p.PenjualanID = &pid
				p.PenjualanTanggal = &pdate
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := base()
			tt.mutate(p)
			err := p.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidMetodeBayar(t *testing.T) {
	cases := map[string]bool{
		"tunai":    true,
		"transfer": true,
		"cek":      true,
		"giro":     true,
		"TUNAI":    true, // case insensitive (ToLower)
		" tunai ":  true, // trimmed
		"":         false,
		"kartu":    false,
		"qris":     false,
	}
	for in, want := range cases {
		got := IsValidMetodeBayar(in)
		if got != want {
			t.Errorf("IsValidMetodeBayar(%q) = %v, want %v", in, got, want)
		}
	}
}

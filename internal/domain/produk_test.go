package domain

import (
	"errors"
	"testing"
)

func TestProduk_Validate(t *testing.T) {
	base := func() *Produk {
		return &Produk{
			SKU:            "SKU-001",
			Nama:           "Semen 50kg",
			SatuanKecilID:  1,
			FaktorKonversi: 1,
			StokMinimum:    0,
		}
	}
	tests := []struct {
		name    string
		mutate  func(p *Produk)
		wantErr error
	}{
		{"ok", func(p *Produk) {}, nil},
		{"sku kosong", func(p *Produk) { p.SKU = "" }, ErrSKUWajib},
		{"sku whitespace", func(p *Produk) { p.SKU = "   " }, ErrSKUWajib},
		{"nama kosong", func(p *Produk) { p.Nama = "" }, ErrNamaWajib},
		{"nama whitespace", func(p *Produk) { p.Nama = "  " }, ErrNamaWajib},
		{"satuan kecil nol", func(p *Produk) { p.SatuanKecilID = 0 }, ErrSatuanKecilWajib},
		{"satuan kecil negatif", func(p *Produk) { p.SatuanKecilID = -1 }, ErrSatuanKecilWajib},
		{"faktor konversi nol", func(p *Produk) { p.FaktorKonversi = 0 }, ErrFaktorKonversiInvalid},
		{"faktor konversi negatif", func(p *Produk) { p.FaktorKonversi = -1.5 }, ErrFaktorKonversiInvalid},
		{"stok minimum negatif", func(p *Produk) { p.StokMinimum = -0.1 }, ErrStokMinimumInvalid},
		{"stok minimum nol ok", func(p *Produk) { p.StokMinimum = 0 }, nil},
		{"faktor konversi pecahan ok", func(p *Produk) { p.FaktorKonversi = 0.5 }, nil},
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

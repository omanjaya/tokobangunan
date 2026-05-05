package domain

import (
	"errors"
	"testing"
)

func TestMitra_Validate(t *testing.T) {
	base := func() *Mitra {
		return &Mitra{
			Kode:           "MIT-001",
			Nama:           "Toko Sumber Jaya",
			Tipe:           MitraTipeEceran,
			LimitKredit:    0,
			JatuhTempoHari: 0,
		}
	}
	tests := []struct {
		name    string
		mutate  func(m *Mitra)
		wantErr error
	}{
		{"ok eceran", func(m *Mitra) {}, nil},
		{"ok grosir", func(m *Mitra) { m.Tipe = MitraTipeGrosir }, nil},
		{"ok proyek", func(m *Mitra) { m.Tipe = MitraTipeProyek }, nil},
		{"kode kosong", func(m *Mitra) { m.Kode = "" }, ErrMitraKodeKosong},
		{"kode whitespace", func(m *Mitra) { m.Kode = "  " }, ErrMitraKodeKosong},
		{"nama kosong", func(m *Mitra) { m.Nama = "" }, ErrMitraNamaKosong},
		{"tipe invalid", func(m *Mitra) { m.Tipe = "vip" }, ErrMitraTipeInvalid},
		{"tipe kosong", func(m *Mitra) { m.Tipe = "" }, ErrMitraTipeInvalid},
		{"limit negatif", func(m *Mitra) { m.LimitKredit = -1 }, ErrMitraLimitNegatif},
		{"tempo negatif", func(m *Mitra) { m.JatuhTempoHari = -1 }, ErrMitraTempoNegatif},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.mutate(m)
			err := m.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidMitraTipe(t *testing.T) {
	cases := map[string]bool{
		"eceran": true,
		"grosir": true,
		"proyek": true,
		"":       false,
		"vip":    false,
		"ECERAN": false, // case sensitive
	}
	for in, want := range cases {
		if got := IsValidMitraTipe(in); got != want {
			t.Errorf("IsValidMitraTipe(%q) = %v, want %v", in, got, want)
		}
	}
}

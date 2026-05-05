package terbilang

import "testing"

func TestKonversi(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "Nol rupiah"},
		{1, "Satu rupiah"},
		{10, "Sepuluh rupiah"},
		{11, "Sebelas rupiah"},
		{15, "Lima belas rupiah"},
		{19, "Sembilan belas rupiah"},
		{20, "Dua puluh rupiah"},
		{21, "Dua puluh satu rupiah"},
		{99, "Sembilan puluh sembilan rupiah"},
		{100, "Seratus rupiah"},
		{101, "Seratus satu rupiah"},
		{200, "Dua ratus rupiah"},
		{999, "Sembilan ratus sembilan puluh sembilan rupiah"},
		{1000, "Seribu rupiah"},
		{1001, "Seribu satu rupiah"},
		{1500, "Seribu lima ratus rupiah"},
		{2000, "Dua ribu rupiah"},
		{12500, "Dua belas ribu lima ratus rupiah"},
		{1_000_000, "Satu juta rupiah"},
		{1_250_000, "Satu juta dua ratus lima puluh ribu rupiah"},
		{1_000_000_000, "Satu miliar rupiah"},
		{2_500_000_000, "Dua miliar lima ratus juta rupiah"},
		{1_000_000_000_000, "Satu triliun rupiah"},
	}
	for _, c := range cases {
		got := Konversi(c.in)
		if got != c.want {
			t.Errorf("Konversi(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKonversiNegative(t *testing.T) {
	got := Konversi(-1500)
	want := "Minus seribu lima ratus rupiah"
	if got != want {
		t.Errorf("Konversi(-1500) = %q, want %q", got, want)
	}
}

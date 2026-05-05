package format

import "testing"

func TestRupiah(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{"nol", 0, "Rp 0"},
		{"100 cents = 1 rupiah", 100, "Rp 1"},
		{"99 cents = 0 rupiah (truncated)", 99, "Rp 0"},
		{"100_000 cents = 1.000 rupiah", 100_000, "Rp 1.000"},
		{"1_500_000 cents = 15.000 rupiah", 1_500_000, "Rp 15.000"},
		{"100_000_000 cents = 1.000.000", 100_000_000, "Rp 1.000.000"},
		{"99_900 = 999", 99_900, "Rp 999"},
		{"100_000_000_000 = 1.000.000.000", 100_000_000_000, "Rp 1.000.000.000"},
		{"negatif kecil", -100, "Rp -1"},
		{"negatif ribuan", -1_500_000, "Rp -15.000"},
		{"negatif jutaan", -100_000_000, "Rp -1.000.000"},
		{"123456 cents = 1.234", 123_456, "Rp 1.234"},
		{"12345678 cents = 123.456", 12_345_678, "Rp 123.456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rupiah(tt.cents)
			if got != tt.want {
				t.Errorf("Rupiah(%d) = %q, want %q", tt.cents, got, tt.want)
			}
		})
	}
}

func TestRupiahShort(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0"},
		{100_000, "1.000"},
		{1_500_000, "15.000"},
		{-100, "-1"},
	}
	for _, tt := range tests {
		got := RupiahShort(tt.cents)
		if got != tt.want {
			t.Errorf("RupiahShort(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

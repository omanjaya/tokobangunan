package domain

import (
	"errors"
	"testing"
)

func validMutasi() *MutasiGudang {
	return &MutasiGudang{
		NomorMutasi:    "MUT-001",
		GudangAsalID:   1,
		GudangTujuanID: 2,
		Status:         StatusDraft,
		Items: []MutasiItem{
			{ProdukID: 1, SatuanID: 1, Qty: 1, QtyKonversi: 1},
		},
	}
}

func TestMutasi_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *MutasiGudang)
		wantErr error
	}{
		{"ok", func(m *MutasiGudang) {}, nil},
		{"nomor kosong", func(m *MutasiGudang) { m.NomorMutasi = "" }, ErrNomorMutasiKosong},
		{"nomor whitespace", func(m *MutasiGudang) { m.NomorMutasi = "   " }, ErrNomorMutasiKosong},
		{"asal sama tujuan", func(m *MutasiGudang) { m.GudangTujuanID = 1 }, ErrAsalSamaTujuan},
		{"asal nol", func(m *MutasiGudang) { m.GudangAsalID = 0 }, ErrAsalSamaTujuan},
		{"tujuan negatif", func(m *MutasiGudang) { m.GudangTujuanID = -1 }, ErrAsalSamaTujuan},
		{"status invalid", func(m *MutasiGudang) { m.Status = StatusMutasi("foo") }, ErrMutasiStatusInvalid},
		{"items kosong", func(m *MutasiGudang) { m.Items = nil }, ErrMutasiKosong},
		{"qty nol", func(m *MutasiGudang) { m.Items[0].Qty = 0 }, ErrMutasiKosong},
		{"qty konversi nol", func(m *MutasiGudang) { m.Items[0].QtyKonversi = 0 }, ErrMutasiKosong},
		{"produk id nol", func(m *MutasiGudang) { m.Items[0].ProdukID = 0 }, ErrMutasiKosong},
		{"satuan id nol", func(m *MutasiGudang) { m.Items[0].SatuanID = 0 }, ErrMutasiKosong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMutasi()
			tt.mutate(m)
			err := m.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatusMutasi_IsValid(t *testing.T) {
	valid := []StatusMutasi{StatusDraft, StatusDikirim, StatusDiterima, StatusDibatalkan}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if StatusMutasi("foo").IsValid() {
		t.Error("foo should be invalid")
	}
}

func TestMutasi_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from StatusMutasi
		to   StatusMutasi
		want bool
	}{
		{StatusDraft, StatusDikirim, true},
		{StatusDraft, StatusDibatalkan, true},
		{StatusDikirim, StatusDiterima, true},
		{StatusDraft, StatusDiterima, false},
		{StatusDikirim, StatusDraft, false},
		{StatusDikirim, StatusDibatalkan, true},
		{StatusDiterima, StatusDraft, false},
		{StatusDiterima, StatusDikirim, false},
		{StatusDiterima, StatusDibatalkan, false},
		{StatusDibatalkan, StatusDraft, false},
		{StatusDibatalkan, StatusDikirim, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%s->%s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

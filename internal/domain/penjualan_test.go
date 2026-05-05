package domain

import (
	"errors"
	"testing"
	"time"
)

func validPenjualan() *Penjualan {
	jt := time.Now().Add(7 * 24 * time.Hour)
	_ = jt
	return &Penjualan{
		Items: []PenjualanItem{
			{ProdukID: 1, Qty: 2, HargaSatuan: 1000, Subtotal: 2000},
		},
		Subtotal:    2000,
		Diskon:      0,
		Total:       2000,
		StatusBayar: StatusLunas,
	}
}

func TestPenjualan_Validate(t *testing.T) {
	jt := time.Now().Add(7 * 24 * time.Hour)
	tests := []struct {
		name    string
		mutate  func(p *Penjualan)
		wantErr error
	}{
		{
			name:    "ok lunas",
			mutate:  func(p *Penjualan) {},
			wantErr: nil,
		},
		{
			name: "items kosong",
			mutate: func(p *Penjualan) {
				p.Items = nil
			},
			wantErr: ErrPenjualanKosong,
		},
		{
			name: "qty item nol",
			mutate: func(p *Penjualan) {
				p.Items[0].Qty = 0
			},
			wantErr: ErrItemQtyInvalid,
		},
		{
			name: "qty item negatif",
			mutate: func(p *Penjualan) {
				p.Items[0].Qty = -1
			},
			wantErr: ErrItemQtyInvalid,
		},
		{
			name: "harga item negatif",
			mutate: func(p *Penjualan) {
				p.Items[0].HargaSatuan = -1
			},
			wantErr: ErrItemHargaInvalid,
		},
		{
			name: "status invalid",
			mutate: func(p *Penjualan) {
				p.StatusBayar = StatusBayar("foo")
			},
			wantErr: ErrStatusBayarInvalid,
		},
		{
			name: "kredit tanpa jatuh tempo",
			mutate: func(p *Penjualan) {
				p.StatusBayar = StatusKredit
				p.JatuhTempo = nil
			},
			wantErr: ErrJatuhTempoWajib,
		},
		{
			name: "sebagian tanpa jatuh tempo",
			mutate: func(p *Penjualan) {
				p.StatusBayar = StatusSebagian
				p.JatuhTempo = nil
			},
			wantErr: ErrJatuhTempoWajib,
		},
		{
			name: "kredit dengan jatuh tempo ok",
			mutate: func(p *Penjualan) {
				p.StatusBayar = StatusKredit
				p.JatuhTempo = &jt
			},
			wantErr: nil,
		},
		{
			name: "total tidak cocok",
			mutate: func(p *Penjualan) {
				p.Total = 999
			},
			wantErr: ErrTotalTidakCocok,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPenjualan()
			tt.mutate(p)
			err := p.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPenjualan_HitungTotal(t *testing.T) {
	tests := []struct {
		name         string
		items        []PenjualanItem
		diskon       int64
		wantSubtotal int64
		wantTotal    int64
	}{
		{
			name: "single item integer qty",
			items: []PenjualanItem{
				{Qty: 2, HargaSatuan: 1000},
			},
			diskon:       0,
			wantSubtotal: 2000,
			wantTotal:    2000,
		},
		{
			name: "multiple items",
			items: []PenjualanItem{
				{Qty: 2, HargaSatuan: 1000},
				{Qty: 3, HargaSatuan: 500},
			},
			diskon:       0,
			wantSubtotal: 3500,
			wantTotal:    3500,
		},
		{
			name: "with diskon",
			items: []PenjualanItem{
				{Qty: 1, HargaSatuan: 10000},
			},
			diskon:       2000,
			wantSubtotal: 10000,
			wantTotal:    8000,
		},
		{
			name: "qty pecahan dibulatkan",
			items: []PenjualanItem{
				{Qty: 1.5, HargaSatuan: 1000},
			},
			diskon:       0,
			wantSubtotal: 1500,
			wantTotal:    1500,
		},
		{
			name: "diskon negatif dipaksa nol",
			items: []PenjualanItem{
				{Qty: 1, HargaSatuan: 5000},
			},
			diskon:       -100,
			wantSubtotal: 5000,
			wantTotal:    5000,
		},
		{
			name:         "items kosong",
			items:        nil,
			diskon:       0,
			wantSubtotal: 0,
			wantTotal:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Penjualan{Items: tt.items, Diskon: tt.diskon}
			p.HitungTotal()
			if p.Subtotal != tt.wantSubtotal {
				t.Errorf("Subtotal = %d, want %d", p.Subtotal, tt.wantSubtotal)
			}
			if p.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", p.Total, tt.wantTotal)
			}
			// verify per-item Subtotal also written.
			var sum int64
			for _, it := range p.Items {
				sum += it.Subtotal
			}
			if sum != p.Subtotal {
				t.Errorf("sum item subtotals = %d, want %d", sum, p.Subtotal)
			}
		})
	}
}

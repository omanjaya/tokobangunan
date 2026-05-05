package domain

import "time"

// Stok - posisi stok per gudang per produk.
// Qty disimpan dalam satuan_kecil (sesuai produk.satuan_kecil_id).
type Stok struct {
	GudangID  int64
	ProdukID  int64
	Qty       float64
	UpdatedAt time.Time
}

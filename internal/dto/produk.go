package dto

// ProdukCreateInput - input membuat produk baru dari form HTML.
type ProdukCreateInput struct {
	SKU            string  `form:"sku" validate:"required,max=64"`
	Nama           string  `form:"nama" validate:"required,max=200"`
	Kategori       string  `form:"kategori" validate:"max=80"`
	SatuanKecilID  int64   `form:"satuan_kecil_id" validate:"required,gt=0"`
	SatuanBesarID  int64   `form:"satuan_besar_id" validate:"gte=0"`
	FaktorKonversi float64 `form:"faktor_konversi" validate:"required,gt=0"`
	StokMinimum    float64 `form:"stok_minimum" validate:"gte=0"`
	IsActive       bool    `form:"is_active"`
	LeadTimeDays   int     `form:"lead_time_days" validate:"gte=0"`
	SafetyStock    float64 `form:"safety_stock" validate:"gte=0"`
}

// ProdukUpdateInput - input update produk. ID datang dari path param, bukan form.
// Version dipakai untuk optimistic concurrency check (0 = skip check).
type ProdukUpdateInput struct {
	SKU            string  `form:"sku" validate:"required,max=64"`
	Nama           string  `form:"nama" validate:"required,max=200"`
	Kategori       string  `form:"kategori" validate:"max=80"`
	SatuanKecilID  int64   `form:"satuan_kecil_id" validate:"required,gt=0"`
	SatuanBesarID  int64   `form:"satuan_besar_id" validate:"gte=0"`
	FaktorKonversi float64 `form:"faktor_konversi" validate:"required,gt=0"`
	StokMinimum    float64 `form:"stok_minimum" validate:"gte=0"`
	IsActive       bool    `form:"is_active"`
	LeadTimeDays   int     `form:"lead_time_days" validate:"gte=0"`
	SafetyStock    float64 `form:"safety_stock" validate:"gte=0"`
	Version        int64   `form:"version" validate:"gte=0"`
}

package dto

// PembelianItemInput satu baris item pada form pembelian.
type PembelianItemInput struct {
	ProdukID    int64   `validate:"required,min=1"`
	Qty         float64 `validate:"required,gt=0"`
	SatuanID    int64   `validate:"required,min=1"`
	HargaSatuan int64   `validate:"required,min=0"` // Rupiah utuh (akan dikonversi ke cents di service)
}

// PembelianCreateInput payload form pembuatan pembelian.
type PembelianCreateInput struct {
	Tanggal     string               `form:"tanggal" validate:"required,datetime=2006-01-02"`
	SupplierID  int64                `form:"supplier_id" validate:"required,min=1"`
	GudangID    int64                `form:"gudang_id" validate:"required,min=1"`
	Items       []PembelianItemInput `validate:"required,min=1,dive"`
	Diskon      int64                `form:"diskon"` // Rupiah utuh
	StatusBayar string               `form:"status_bayar" validate:"required,oneof=lunas kredit sebagian"`
	JatuhTempo  string               `form:"jatuh_tempo"`
	Catatan     string               `form:"catatan" validate:"max=1024"`
}

// PembayaranSupplierInput payload form pembayaran supplier.
type PembayaranSupplierInput struct {
	PembelianID *int64 `form:"pembelian_id"`
	SupplierID  int64  `form:"supplier_id" validate:"required,min=1"`
	Tanggal     string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah      int64  `form:"jumlah" validate:"required,gt=0"` // Rupiah utuh
	Metode      string `form:"metode" validate:"required,oneof=tunai transfer cek giro"`
	Referensi   string `form:"referensi" validate:"max=128"`
	Catatan     string `form:"catatan" validate:"max=512"`
}

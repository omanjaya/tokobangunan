package dto

// PenjualanItemInput - 1 baris item di form penjualan.
// Harga satuan dalam Rupiah utuh (service akan konversi ke cents).
type PenjualanItemInput struct {
	ProdukID    int64   `form:"produk_id" validate:"required,min=1"`
	Qty         float64 `form:"qty" validate:"required,gt=0"`
	SatuanID    int64   `form:"satuan_id" validate:"required,min=1"`
	HargaSatuan int64   `form:"harga_satuan" validate:"min=0"` // Rupiah utuh
	Diskon      int64   `form:"diskon" validate:"min=0"`       // Rupiah utuh
}

// PenjualanCreateInput - input membuat penjualan baru.
// Tanggal format YYYY-MM-DD; JatuhTempo opsional (sama format).
// Diskon dalam Rupiah utuh.
type PenjualanCreateInput struct {
	Tanggal     string               `form:"tanggal" validate:"required,datetime=2006-01-02"`
	MitraID     int64                `form:"mitra_id" validate:"required,min=1"`
	GudangID    int64                `form:"gudang_id" validate:"required,min=1"`
	Items       []PenjualanItemInput `validate:"required,min=1,dive"`
	Diskon      int64                `form:"diskon" validate:"min=0"`
	StatusBayar string               `form:"status_bayar" validate:"required,oneof=lunas kredit sebagian"`
	JatuhTempo  string               `form:"jatuh_tempo"`
	Catatan     string               `form:"catatan"`
	ClientUUID  string               `form:"client_uuid"`

	// PPNEnabled toggle PPN; persentase diambil dari app_setting.pajak_config.
	PPNEnabled bool `form:"ppn_enabled"`
}

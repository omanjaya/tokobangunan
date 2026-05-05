package dto

// PembayaranCreateInput input form pencatatan pembayaran customer.
// PenjualanID nil → pembayaran umum (akan dialokasi/disimpan tanpa link).
type PembayaranCreateInput struct {
	PenjualanID *int64 `form:"penjualan_id"`
	MitraID     int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal     string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah      int64  `form:"jumlah" validate:"required,gt=0"` // Rupiah utuh
	Metode      string `form:"metode" validate:"required,oneof=tunai transfer cek giro"`
	Referensi   string `form:"referensi" validate:"max=128"`
	Catatan     string `form:"catatan" validate:"max=512"`
	ClientUUID  string `form:"client_uuid"`
}

// PembayaranBatchInput input form pembayaran batch (FIFO ke invoice tertua).
type PembayaranBatchInput struct {
	MitraID    int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal    string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah     int64  `form:"jumlah" validate:"required,gt=0"`
	Metode     string `form:"metode" validate:"required,oneof=tunai transfer cek giro"`
	Referensi  string `form:"referensi" validate:"max=128"`
	Catatan    string `form:"catatan" validate:"max=512"`
	ClientUUID string `form:"client_uuid"`
}

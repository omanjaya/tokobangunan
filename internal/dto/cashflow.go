package dto

// CashflowCreateInput - input membuat cashflow baru.
// Jumlah dalam Rupiah utuh (handler akan konversi ke cents).
type CashflowCreateInput struct {
	Tanggal   string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	GudangID  *int64 `form:"gudang_id"`
	Tipe      string `form:"tipe" validate:"required,oneof=masuk keluar"`
	Kategori  string `form:"kategori" validate:"required"`
	Deskripsi string `form:"deskripsi"`
	Jumlah    int64  `form:"jumlah" validate:"required,gt=0"` // Rupiah utuh
	Metode    string `form:"metode" validate:"required,oneof=tunai transfer cek"`
	Referensi string `form:"referensi"`
	Catatan   string `form:"catatan"`
}

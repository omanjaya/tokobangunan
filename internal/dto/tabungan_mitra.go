package dto

// TabunganSetorInput payload setor (debit) tabungan mitra.
type TabunganSetorInput struct {
	MitraID int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah  int64  `form:"jumlah" validate:"required,gt=0"` // Rupiah utuh
	Catatan string `form:"catatan" validate:"max=512"`
}

// TabunganTarikInput payload tarik (kredit) tabungan mitra.
type TabunganTarikInput struct {
	MitraID int64  `form:"mitra_id" validate:"required,min=1"`
	Tanggal string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	Jumlah  int64  `form:"jumlah" validate:"required,gt=0"`
	Catatan string `form:"catatan" validate:"max=512"`
}

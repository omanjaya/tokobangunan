package dto

// StokOpnameCreateInput payload pembuatan opname (header saja, item di-prefill).
type StokOpnameCreateInput struct {
	Tanggal  string `form:"tanggal" validate:"required,datetime=2006-01-02"`
	GudangID int64  `form:"gudang_id" validate:"required,min=1"`
	Catatan  string `form:"catatan" validate:"max=1024"`
}

// StokOpnameItemInput payload update qty fisik per item.
type StokOpnameItemInput struct {
	QtyFisik   float64 `form:"qty_fisik" validate:"gte=0"`
	Keterangan string  `form:"keterangan" validate:"max=512"`
}

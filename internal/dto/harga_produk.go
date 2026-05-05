package dto

// HargaSetInput - set harga baru pada history.
// HargaJual diterima dalam Rupiah utuh (bukan cents) untuk kemudahan input;
// service akan multiply ke cents.
type HargaSetInput struct {
	GudangID    int64  `form:"gudang_id" validate:"gte=0"`
	Tipe        string `form:"tipe" validate:"required,oneof=eceran grosir proyek"`
	HargaJual   int64  `form:"harga_jual" validate:"required,gt=0"`
	BerlakuDari string `form:"berlaku_dari" validate:"required"`
}

package dto

// SatuanCreateInput - input untuk tambah satuan baru.
type SatuanCreateInput struct {
	Kode string `form:"kode" validate:"required,max=16,lowercase"`
	Nama string `form:"nama" validate:"required,max=80"`
}

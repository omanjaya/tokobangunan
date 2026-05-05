package dto

// SupplierCreateInput - form input pembuatan supplier.
type SupplierCreateInput struct {
	Kode     string `form:"kode" validate:"required,max=32"`
	Nama     string `form:"nama" validate:"required,max=128"`
	Alamat   string `form:"alamat" validate:"max=512"`
	Kontak   string `form:"kontak" validate:"max=64"`
	Catatan  string `form:"catatan" validate:"max=1024"`
	IsActive bool   `form:"is_active"`
}

// SupplierUpdateInput - sama dengan create + ID dari path.
type SupplierUpdateInput struct {
	ID       int64  `param:"id" validate:"required"`
	Kode     string `form:"kode" validate:"required,max=32"`
	Nama     string `form:"nama" validate:"required,max=128"`
	Alamat   string `form:"alamat" validate:"max=512"`
	Kontak   string `form:"kontak" validate:"max=64"`
	Catatan  string `form:"catatan" validate:"max=1024"`
	IsActive bool   `form:"is_active"`
}

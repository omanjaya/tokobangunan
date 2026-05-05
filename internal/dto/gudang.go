package dto

// GudangCreateInput - data form create gudang.
type GudangCreateInput struct {
	Kode     string `validate:"required,max=32"`
	Nama     string `validate:"required,max=128"`
	Alamat   string `validate:"max=255"`
	Telepon  string `validate:"max=32"`
	IsActive bool
}

// GudangUpdateInput - data form update gudang.
type GudangUpdateInput struct {
	Kode     string `validate:"required,max=32"`
	Nama     string `validate:"required,max=128"`
	Alamat   string `validate:"max=255"`
	Telepon  string `validate:"max=32"`
	IsActive bool
}

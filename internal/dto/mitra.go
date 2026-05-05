package dto

// MitraCreateInput - form input pembuatan mitra. Currency dalam rupiah utuh
// (akan dikalikan 100 di service untuk simpan sebagai cents).
type MitraCreateInput struct {
	Kode            string `form:"kode" validate:"required,max=32"`
	Nama            string `form:"nama" validate:"required,max=128"`
	Alamat          string `form:"alamat" validate:"max=512"`
	Kontak          string `form:"kontak" validate:"max=64"`
	NPWP            string `form:"npwp" validate:"max=32"`
	Tipe            string `form:"tipe" validate:"required,oneof=eceran grosir proyek"`
	LimitKreditRp   int64  `form:"limit_kredit" validate:"gte=0"`
	JatuhTempoHari  int    `form:"jatuh_tempo_hari" validate:"gte=0"`
	GudangDefaultID int64  `form:"gudang_default_id" validate:"gte=0"`
	Catatan         string `form:"catatan" validate:"max=1024"`
	IsActive        bool   `form:"is_active"`
}

// MitraUpdateInput - sama dengan create + ID dari path.
// Version dipakai untuk optimistic concurrency check (0 = skip).
type MitraUpdateInput struct {
	ID              int64  `param:"id" validate:"required"`
	Kode            string `form:"kode" validate:"required,max=32"`
	Nama            string `form:"nama" validate:"required,max=128"`
	Alamat          string `form:"alamat" validate:"max=512"`
	Kontak          string `form:"kontak" validate:"max=64"`
	NPWP            string `form:"npwp" validate:"max=32"`
	Tipe            string `form:"tipe" validate:"required,oneof=eceran grosir proyek"`
	LimitKreditRp   int64  `form:"limit_kredit" validate:"gte=0"`
	JatuhTempoHari  int    `form:"jatuh_tempo_hari" validate:"gte=0"`
	GudangDefaultID int64  `form:"gudang_default_id" validate:"gte=0"`
	Catatan         string `form:"catatan" validate:"max=1024"`
	IsActive        bool   `form:"is_active"`
	Version         int64  `form:"version" validate:"gte=0"`
}

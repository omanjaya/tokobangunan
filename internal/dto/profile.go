package dto

// ProfileUpdateInput - data form update profil pribadi (self-service).
// Tidak include username/role/gudang — itu hanya bisa diubah admin lewat
// modul setting/user.
type ProfileUpdateInput struct {
	NamaLengkap string `validate:"required,max=128"`
	Email       string `validate:"omitempty,email,max=128"`
}

package dto

// UserCreateInput - data form create user. Password digenerate oleh service.
type UserCreateInput struct {
	Username    string `validate:"required,max=32"`
	NamaLengkap string `validate:"required,max=128"`
	Email       string `validate:"omitempty,email,max=128"`
	Role        string `validate:"required,oneof=owner admin kasir gudang"`
	GudangID    *int64
	IsActive    bool
}

// UserUpdateInput - data form update user (tanpa password).
type UserUpdateInput struct {
	Username    string `validate:"required,max=32"`
	NamaLengkap string `validate:"required,max=128"`
	Email       string `validate:"omitempty,email,max=128"`
	Role        string `validate:"required,oneof=owner admin kasir gudang"`
	GudangID    *int64
	IsActive    bool
}

// ChangePasswordInput - user ganti password sendiri.
type ChangePasswordInput struct {
	OldPassword string `validate:"required"`
	NewPassword string `validate:"required,min=8,max=72"`
}

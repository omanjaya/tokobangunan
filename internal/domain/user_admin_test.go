package domain

import (
	"errors"
	"testing"
)

func TestUserAccount_Validate(t *testing.T) {
	base := func() *UserAccount {
		return &UserAccount{
			Username:    "kasir01",
			NamaLengkap: "Kasir Satu",
			Role:        RoleKasir,
		}
	}
	tests := []struct {
		name    string
		mutate  func(u *UserAccount)
		wantErr error
	}{
		{"ok kasir", func(u *UserAccount) {}, nil},
		{"ok owner", func(u *UserAccount) { u.Role = RoleOwner }, nil},
		{"ok admin", func(u *UserAccount) { u.Role = RoleAdmin }, nil},
		{"ok gudang", func(u *UserAccount) { u.Role = RoleGudang }, nil},
		{"username kosong", func(u *UserAccount) { u.Username = "" }, ErrUserUsernameWajib},
		{"username whitespace", func(u *UserAccount) { u.Username = "   " }, ErrUserUsernameWajib},
		{"username terlalu pendek", func(u *UserAccount) { u.Username = "ab" }, ErrUserUsernameFormat},
		{"username angka di awal", func(u *UserAccount) { u.Username = "1abc" }, ErrUserUsernameFormat},
		{"username uppercase", func(u *UserAccount) { u.Username = "Kasir01" }, ErrUserUsernameFormat},
		{"username dash", func(u *UserAccount) { u.Username = "kasir-01" }, ErrUserUsernameFormat},
		{"username terlalu panjang", func(u *UserAccount) {
			u.Username = "abcdefghijklmnopqrstuvwxyz0123456789" // 36 chars
		}, ErrUserUsernameFormat},
		{"nama kosong", func(u *UserAccount) { u.NamaLengkap = "" }, ErrUserNamaWajib},
		{"role invalid", func(u *UserAccount) { u.Role = "superuser" }, ErrUserRoleInvalid},
		{"role kosong", func(u *UserAccount) { u.Role = "" }, ErrUserRoleInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := base()
			tt.mutate(u)
			err := u.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidUserRole(t *testing.T) {
	cases := map[string]bool{
		"owner":  true,
		"admin":  true,
		"kasir":  true,
		"gudang": true,
		"":       false,
		"super":  false,
		"OWNER":  false,
	}
	for in, want := range cases {
		if got := IsValidUserRole(in); got != want {
			t.Errorf("IsValidUserRole(%q) = %v, want %v", in, got, want)
		}
	}
}

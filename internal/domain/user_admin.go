package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Role enum.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleKasir  = "kasir"
	RoleGudang = "gudang"
)

var (
	ErrUserUsernameWajib    = errors.New("username wajib diisi")
	ErrUserUsernameFormat   = errors.New("username harus 3-31 karakter, huruf kecil/digit/underscore, diawali huruf")
	ErrUserNamaWajib        = errors.New("nama lengkap wajib diisi")
	ErrUserRoleInvalid      = errors.New("role harus owner/admin/kasir/gudang")
	ErrUserAccountNotFound  = errors.New("user tidak ditemukan")
	ErrUserUsernameDuplikat = errors.New("username sudah dipakai")
	ErrUserPasswordLemah    = errors.New("password minimal 8 karakter")
	ErrUserPasswordSalah    = errors.New("password lama salah")
)

var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{2,30}$`)

// UserAccount domain entity untuk modul management. Tidak expose
// password_hash, failed_attempts, locked_until — itu internal auth.
type UserAccount struct {
	ID          int64
	Username    string
	NamaLengkap string
	Email       *string
	Role        string
	GudangID    *int64
	IsActive    bool
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsValidUserRole true bila string adalah role yang dikenali.
func IsValidUserRole(r string) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleKasir, RoleGudang:
		return true
	}
	return false
}

// Validate cek invariant entity.
func (u *UserAccount) Validate() error {
	username := strings.TrimSpace(u.Username)
	if username == "" {
		return ErrUserUsernameWajib
	}
	if !usernameRegex.MatchString(username) {
		return ErrUserUsernameFormat
	}
	if strings.TrimSpace(u.NamaLengkap) == "" {
		return ErrUserNamaWajib
	}
	if !IsValidUserRole(u.Role) {
		return ErrUserRoleInvalid
	}
	return nil
}

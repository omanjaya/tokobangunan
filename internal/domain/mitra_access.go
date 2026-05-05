package domain

import (
	"errors"
	"time"
)

// Sentinel error mitra access token.
var (
	ErrAccessTokenNotFound = errors.New("access token tidak ditemukan")
	ErrAccessTokenInvalid  = errors.New("access token invalid")
	ErrAccessTokenRevoked  = errors.New("access token sudah dicabut")
	ErrAccessTokenExpired  = errors.New("access token sudah kedaluwarsa")
)

// MitraAccessToken token magic-link untuk customer portal mitra.
type MitraAccessToken struct {
	ID        int64
	MitraID   int64
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
}

// IsValid cek token belum expired & belum revoke.
func (t *MitraAccessToken) IsValid() error {
	if t.Revoked {
		return ErrAccessTokenRevoked
	}
	if time.Now().After(t.ExpiresAt) {
		return ErrAccessTokenExpired
	}
	return nil
}

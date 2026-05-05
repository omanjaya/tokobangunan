package crypto

import (
	"crypto/sha256"
	"io"
	"os"

	"golang.org/x/crypto/hkdf"
)

// DeriveKey turunkan 32-byte subkey dari masterSecret pakai HKDF-SHA256
// dengan domain separation lewat purpose. Info string mengikat ke versi
// aplikasi sehingga rotasi format ke depan tidak collision.
//
// Purpose yang dipakai sekarang: "session", "secret-encrypt", "share-token".
func DeriveKey(masterSecret []byte, purpose string) []byte {
	info := []byte("tokobangunan/v1/" + purpose)
	r := hkdf.New(sha256.New, masterSecret, nil, info)
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		// HKDF dari sha256 dengan output 32 byte tidak akan gagal kecuali
		// reader bocor — fallback aman: return zero (caller akan fail downstream).
		return make([]byte, 32)
	}
	return out
}

// DeriveKeyFromEnv ambil SESSION_SECRET dari env lalu derive untuk purpose.
// Return nil + false kalau env kosong (caller wajib handle).
func DeriveKeyFromEnv(purpose string) ([]byte, bool) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, false
	}
	return DeriveKey([]byte(secret), purpose), true
}

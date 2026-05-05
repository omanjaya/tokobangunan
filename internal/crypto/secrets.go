// Package crypto menyediakan helper enkripsi simetris untuk secret-at-rest.
//
// Skema: AES-256-GCM, key = SHA-256(SESSION_SECRET).
// Output adalah base64(nonce || ciphertext+tag).
//
// Backwards compat: caller dapat memanggil DecryptSecret pada nilai lama
// yang mungkin masih plaintext; kalau decode/decrypt gagal, treat sebagai
// legacy plaintext (lihat DecryptSecretCompat).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// ErrEmptyKey - SESSION_SECRET belum di-set.
var ErrEmptyKey = errors.New("SESSION_SECRET kosong; tidak bisa derive key")

// deriveKey turunkan 32-byte key dari SESSION_SECRET via SHA-256.
func deriveKey() ([]byte, error) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, ErrEmptyKey
	}
	h := sha256.Sum256([]byte(secret))
	return h[:], nil
}

// EncryptSecret enkripsi plaintext dengan AES-GCM, output base64.
// String kosong tetap dikembalikan kosong (tidak perlu encrypt).
func EncryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := deriveKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptSecret dekripsi hasil EncryptSecret. String kosong → "".
// Error bila ciphertext invalid atau key salah.
func DecryptSecret(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	key, err := deriveKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext terlalu pendek")
	}
	nonce := raw[:gcm.NonceSize()]
	ct := raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// DecryptSecretCompat: coba decrypt; kalau gagal, return value asli sebagai
// legacy plaintext beserta isLegacy=true. Caller bisa memilih untuk
// re-encrypt pada save berikutnya.
func DecryptSecretCompat(stored string) (plain string, isLegacy bool) {
	if stored == "" {
		return "", false
	}
	if pt, err := DecryptSecret(stored); err == nil {
		return pt, false
	}
	return stored, true
}

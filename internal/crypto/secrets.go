// Package crypto menyediakan helper enkripsi simetris untuk secret-at-rest.
//
// Skema v1: AES-256-GCM, key = HKDF-SHA256(SESSION_SECRET, "secret-encrypt").
// Output adalah "enc:v1:" + base64(nonce || ciphertext+tag).
//
// Backwards compat:
//   - Stored value tanpa magic prefix "enc:v1:" diasumsikan legacy plaintext
//     dan dikembalikan apa adanya (DecryptSecret + DecryptSecretCompat).
//   - Stored value dengan prefix tetapi gagal decrypt (ciphertext tampered
//     atau key salah) → return error (anti-tamper).
//
// TODO(migration): Existing encrypted SMTP password (kalau ada di DB) memakai
// skema lama (SHA256(SESSION_SECRET) + tanpa prefix). Setelah deploy versi ini
// nilai itu akan terbaca sebagai "legacy plaintext" oleh DecryptSecretCompat —
// caller harus re-save (panggil EncryptSecret ulang) untuk migrate ke v1.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// ErrEmptyKey - SESSION_SECRET belum di-set.
var ErrEmptyKey = errors.New("SESSION_SECRET kosong; tidak bisa derive key")

// ErrTampered - prefix v1 hadir tapi ciphertext invalid (tampered atau key beda).
var ErrTampered = errors.New("ciphertext v1 invalid: tampered atau key mismatch")

const encV1Prefix = "enc:v1:"

// deriveKey turunkan 32-byte key untuk secret-encrypt purpose via HKDF.
func deriveKey() ([]byte, error) {
	key, ok := DeriveKeyFromEnv("secret-encrypt")
	if !ok {
		return nil, ErrEmptyKey
	}
	return key, nil
}

// EncryptSecret enkripsi plaintext dengan AES-GCM, output "enc:v1:" + base64.
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
	return encV1Prefix + base64.StdEncoding.EncodeToString(out), nil
}

// DecryptSecret dekripsi hasil EncryptSecret.
//   - "" → ""
//   - tanpa prefix "enc:v1:" → return apa adanya (legacy plaintext compat)
//   - dengan prefix tetapi gagal decrypt → ErrTampered
func DecryptSecret(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, encV1Prefix) {
		// Legacy plaintext — return as-is.
		return stored, nil
	}
	body := strings.TrimPrefix(stored, encV1Prefix)
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return "", ErrTampered
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
		return "", ErrTampered
	}
	nonce := raw[:gcm.NonceSize()]
	ct := raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrTampered
	}
	return string(pt), nil
}

// DecryptSecretCompat: convenience wrapper. Nilai tanpa prefix dianggap
// legacy plaintext (isLegacy=true). Caller bisa pakai sinyal ini untuk
// re-encrypt pada save berikutnya.
func DecryptSecretCompat(stored string) (plain string, isLegacy bool) {
	if stored == "" {
		return "", false
	}
	if !strings.HasPrefix(stored, encV1Prefix) {
		return stored, true
	}
	pt, err := DecryptSecret(stored)
	if err != nil {
		// Tampered v1 — jangan fallback ke plaintext, kembalikan kosong.
		return "", false
	}
	return pt, false
}

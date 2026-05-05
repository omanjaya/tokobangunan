package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"

	"golang.org/x/crypto/argon2"
)

// Argon2id params dipakai HANYA saat HashPassword (hash baru).
// VerifyPassword baca params dari PHC string sehingga hash lama (t=3) tetap verify OK.
const (
	argonMemory     = 64 * 1024 // 64 MiB
	argonIterations = 4
	argonParallel   = 2
	argonKeyLen     = 32
	argonSaltLen    = 16
)

// HashPassword menghasilkan encoded argon2id PHC string dari plaintext.
// Format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func HashPassword(plaintext string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	hash := argon2.IDKey([]byte(plaintext), salt, argonIterations, argonMemory, argonParallel, argonKeyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonIterations, argonParallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))
	return encoded, nil
}

// GenerateRandomPassword menghasilkan password alfanumerik mudah dibaca
// (tanpa karakter ambigu seperti 0/O, 1/l/I) sepanjang length.
func GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("rand int: %w", err)
		}
		out[i] = chars[n.Int64()]
	}
	return string(out), nil
}

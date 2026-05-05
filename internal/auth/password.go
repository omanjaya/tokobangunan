package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("format hash argon2id tidak valid")
	ErrIncompatibleVersion = errors.New("versi argon2id tidak kompatibel")
)

// VerifyPassword membandingkan password plaintext dengan encoded hash argon2id
// dalam format PHC: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	keyLen := uint32(len(expectedHash))
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)

	return subtle.ConstantTimeCompare(expectedHash, computed) == 1, nil
}

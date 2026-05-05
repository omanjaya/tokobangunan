package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifyPassword_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		wantErr error
	}{
		{"empty", "", ErrInvalidHash},
		{"too few parts", "$argon2id$v=19$m=64,t=3,p=2", ErrInvalidHash},
		{"too many parts", "$argon2id$v=19$m=64,t=3,p=2$abc$def$xyz", ErrInvalidHash},
		{"wrong algorithm", "$bcrypt$v=19$m=64,t=3,p=2$YWJj$ZGVm", ErrInvalidHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyPassword("pw", tt.encoded)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyPassword_IncompatibleVersion(t *testing.T) {
	// Valid format with wrong version.
	encoded := "$argon2id$v=18$m=65536,t=3,p=2$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcGFiY2RlZmdoaWprbG1ub3A"
	_, err := VerifyPassword("pw", encoded)
	if !errors.Is(err, ErrIncompatibleVersion) {
		t.Errorf("err = %v, want ErrIncompatibleVersion", err)
	}
}

func TestVerifyPassword_BadVersionParse(t *testing.T) {
	encoded := "$argon2id$vXX$m=65536,t=3,p=2$YWJj$ZGVm"
	_, err := VerifyPassword("pw", encoded)
	if err == nil {
		t.Error("expected error parsing version")
	}
}

func TestVerifyPassword_BadParams(t *testing.T) {
	encoded := "$argon2id$v=19$bogus$YWJj$ZGVm"
	_, err := VerifyPassword("pw", encoded)
	if err == nil {
		t.Error("expected error parsing params")
	}
}

func TestVerifyPassword_BadSalt(t *testing.T) {
	encoded := "$argon2id$v=19$m=65536,t=3,p=2$!!!notbase64$YWJj"
	_, err := VerifyPassword("pw", encoded)
	if err == nil {
		t.Error("expected error decoding salt")
	}
	if !strings.Contains(err.Error(), "salt") {
		t.Errorf("err should mention salt, got %v", err)
	}
}

func TestVerifyPassword_BadHash(t *testing.T) {
	encoded := "$argon2id$v=19$m=65536,t=3,p=2$YWJjZGVm$!!!notbase64"
	_, err := VerifyPassword("pw", encoded)
	if err == nil {
		t.Error("expected error decoding hash")
	}
}

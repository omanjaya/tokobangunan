package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	pw := "rahasia-banget-123"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=") {
		t.Errorf("hash prefix invalid: %s", h)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 PHC parts, got %d (%q)", len(parts), h)
	}
	ok, err := VerifyPassword(pw, h)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("expected verify true for matching password")
	}
	ok, err = VerifyPassword("wrong", h)
	if err != nil {
		t.Fatalf("VerifyPassword wrong: %v", err)
	}
	if ok {
		t.Error("expected verify false for mismatched password")
	}
}

func TestHashPassword_DifferentSaltEachCall(t *testing.T) {
	pw := "samepassword"
	h1, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("two hashes of same password should differ (random salt)")
	}
}

func TestGenerateRandomPassword_Length(t *testing.T) {
	tests := []int{1, 8, 16, 32, 64}
	for _, l := range tests {
		got, err := GenerateRandomPassword(l)
		if err != nil {
			t.Fatalf("GenerateRandomPassword(%d): %v", l, err)
		}
		if len(got) != l {
			t.Errorf("len = %d, want %d", len(got), l)
		}
	}
}

func TestGenerateRandomPassword_DefaultLength(t *testing.T) {
	got, err := GenerateRandomPassword(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 16 {
		t.Errorf("default length = %d, want 16", len(got))
	}
	got2, err := GenerateRandomPassword(-5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 16 {
		t.Errorf("negative length should default to 16, got %d", len(got2))
	}
}

func TestGenerateRandomPassword_CharsetWhitelist(t *testing.T) {
	const allowed = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for i := 0; i < 50; i++ {
		got, err := GenerateRandomPassword(20)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range got {
			if !strings.ContainsRune(allowed, c) {
				t.Errorf("invalid char %q in %q", c, got)
			}
		}
	}
}

func TestGenerateRandomPassword_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		got, err := GenerateRandomPassword(16)
		if err != nil {
			t.Fatal(err)
		}
		if seen[got] {
			t.Errorf("duplicate password generated: %q", got)
		}
		seen[got] = true
	}
}

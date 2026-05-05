package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrShareTokenInvalid - sentinel.
var (
	ErrShareTokenInvalid = errors.New("share token invalid")
	ErrShareTokenExpired = errors.New("share token expired")
)

// shareEnvelope payload internal.
type shareEnvelope struct {
	Payload json.RawMessage `json:"p"`
	Exp     int64           `json:"exp"`
}

// GenerateShareToken buat token signed (HMAC-SHA256) berisi payload + expiry.
// Format: <base64url(envelope_json)>.<base64url(hmac)>
func GenerateShareToken(secret []byte, payload any, expires time.Time) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	env := shareEnvelope{Payload: raw, Exp: expires.Unix()}
	envJSON, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(envJSON)
	mac := hmacSign(secret, body)
	return body + "." + mac, nil
}

// VerifyShareToken validasi token & unmarshal payload ke dest.
func VerifyShareToken(secret []byte, token string, dest any) error {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return ErrShareTokenInvalid
	}
	body, sig := parts[0], parts[1]
	expectedSig := hmacSign(secret, body)
	if !hmac.Equal([]byte(expectedSig), []byte(sig)) {
		return ErrShareTokenInvalid
	}
	envJSON, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return ErrShareTokenInvalid
	}
	var env shareEnvelope
	if err := json.Unmarshal(envJSON, &env); err != nil {
		return ErrShareTokenInvalid
	}
	if env.Exp > 0 && time.Now().Unix() > env.Exp {
		return ErrShareTokenExpired
	}
	if dest != nil {
		if err := json.Unmarshal(env.Payload, dest); err != nil {
			return ErrShareTokenInvalid
		}
	}
	return nil
}

func hmacSign(secret []byte, body string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// MitraAccessService - issue & verify magic link token mitra.
type MitraAccessService struct {
	repo  *repo.MitraAccessRepo
	mitra *repo.MitraRepo
}

// NewMitraAccessService konstruktor.
func NewMitraAccessService(r *repo.MitraAccessRepo, mr *repo.MitraRepo) *MitraAccessService {
	return &MitraAccessService{repo: r, mitra: mr}
}

// Create generate random token (32 byte hex), insert ke DB. ExpiresDays default 30.
func (s *MitraAccessService) Create(ctx context.Context, mitraID int64, expiresDays int) (*domain.MitraAccessToken, error) {
	if _, err := s.mitra.GetByID(ctx, mitraID); err != nil {
		return nil, err
	}
	if expiresDays <= 0 {
		expiresDays = 30
	}
	token, err := randomToken(32)
	if err != nil {
		return nil, fmt.Errorf("random token: %w", err)
	}
	t := &domain.MitraAccessToken{
		MitraID:   mitraID,
		Token:     token,
		ExpiresAt: time.Now().AddDate(0, 0, expiresDays),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetByToken validasi token aktif & tidak expired, return mitra-side token.
func (s *MitraAccessService) GetByToken(ctx context.Context, token string) (*domain.MitraAccessToken, error) {
	t, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := t.IsValid(); err != nil {
		return nil, err
	}
	return t, nil
}

// Revoke cabut token (id milik mitraID).
func (s *MitraAccessService) Revoke(ctx context.Context, id, mitraID int64) error {
	return s.repo.Revoke(ctx, id, mitraID)
}

// ListByMitra list token milik 1 mitra (untuk admin owner).
func (s *MitraAccessService) ListByMitra(ctx context.Context, mitraID int64) ([]domain.MitraAccessToken, error) {
	return s.repo.ListByMitra(ctx, mitraID)
}

func randomToken(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

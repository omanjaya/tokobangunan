package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// SatuanService - CRUD ringan satuan (master fixed, no delete).
type SatuanService struct {
	repo *repo.SatuanRepo
}

func NewSatuanService(r *repo.SatuanRepo) *SatuanService {
	return &SatuanService{repo: r}
}

func (s *SatuanService) List(ctx context.Context) ([]domain.Satuan, error) {
	return s.repo.List(ctx)
}

func (s *SatuanService) Get(ctx context.Context, id int64) (*domain.Satuan, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SatuanService) Create(ctx context.Context, in dto.SatuanCreateInput) (*domain.Satuan, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}
	sat := &domain.Satuan{
		Kode: strings.ToLower(strings.TrimSpace(in.Kode)),
		Nama: strings.TrimSpace(in.Nama),
	}
	if err := sat.Validate(); err != nil {
		return nil, err
	}
	// Cek dupe kode.
	existing, err := s.repo.GetByKode(ctx, sat.Kode)
	if err != nil && !errors.Is(err, domain.ErrSatuanNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrSatuanKodeDuplikat
	}
	if err := s.repo.Create(ctx, sat); err != nil {
		return nil, fmt.Errorf("create satuan: %w", err)
	}
	return sat, nil
}

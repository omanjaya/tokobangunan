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

// GudangService - business logic master gudang.
type GudangService struct {
	repo *repo.GudangRepo
}

func NewGudangService(r *repo.GudangRepo) *GudangService {
	return &GudangService{repo: r}
}

func (s *GudangService) List(ctx context.Context, includeInactive bool) ([]domain.Gudang, error) {
	return s.repo.List(ctx, includeInactive)
}

func (s *GudangService) Get(ctx context.Context, id int64) (*domain.Gudang, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *GudangService) Create(ctx context.Context, in dto.GudangCreateInput) (*domain.Gudang, error) {
	g := &domain.Gudang{
		Kode:     strings.ToUpper(strings.TrimSpace(in.Kode)),
		Nama:     strings.TrimSpace(in.Nama),
		Alamat:   trimToPtr(in.Alamat),
		Telepon:  trimToPtr(in.Telepon),
		IsActive: in.IsActive,
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}

	// Cek dupe kode.
	if existing, err := s.repo.GetByKode(ctx, g.Kode); err == nil && existing != nil {
		return nil, domain.ErrGudangKodeDuplikat
	} else if err != nil && !errors.Is(err, domain.ErrGudangNotFound) {
		return nil, err
	}

	if err := s.repo.Create(ctx, g); err != nil {
		return nil, fmt.Errorf("create gudang: %w", err)
	}
	return g, nil
}

func (s *GudangService) Update(ctx context.Context, id int64, in dto.GudangUpdateInput) (*domain.Gudang, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newKode := strings.ToUpper(strings.TrimSpace(in.Kode))
	if newKode != g.Kode {
		if existing, err := s.repo.GetByKode(ctx, newKode); err == nil && existing != nil && existing.ID != id {
			return nil, domain.ErrGudangKodeDuplikat
		} else if err != nil && !errors.Is(err, domain.ErrGudangNotFound) {
			return nil, err
		}
	}

	g.Kode = newKode
	g.Nama = strings.TrimSpace(in.Nama)
	g.Alamat = trimToPtr(in.Alamat)
	g.Telepon = trimToPtr(in.Telepon)
	g.IsActive = in.IsActive

	if err := g.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, g); err != nil {
		return nil, fmt.Errorf("update gudang: %w", err)
	}
	return g, nil
}

func (s *GudangService) SetActive(ctx context.Context, id int64, active bool) error {
	return s.repo.SetActive(ctx, id, active)
}

// Delete - soft delete via SetActive(false).
func (s *GudangService) Delete(ctx context.Context, id int64) error {
	return s.repo.SetActive(ctx, id, false)
}

func trimToPtr(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}

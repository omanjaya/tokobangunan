package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// DiskonMasterService - business logic master diskon.
type DiskonMasterService struct {
	repo *repo.DiskonMasterRepo
}

func NewDiskonMasterService(r *repo.DiskonMasterRepo) *DiskonMasterService {
	return &DiskonMasterService{repo: r}
}

func (s *DiskonMasterService) List(ctx context.Context, onlyActive bool) ([]domain.DiskonMaster, error) {
	return s.repo.List(ctx, repo.DiskonFilter{OnlyActive: onlyActive})
}

func (s *DiskonMasterService) Get(ctx context.Context, id int64) (*domain.DiskonMaster, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DiskonMasterService) GetByKode(ctx context.Context, kode string) (*domain.DiskonMaster, error) {
	return s.repo.GetByKode(ctx, strings.ToUpper(strings.TrimSpace(kode)))
}

// ListApplicable - diskon yg aktif & berlaku & subtotal memenuhi min_subtotal.
// subtotal dalam cents, t = waktu transaksi (biasanya now()).
func (s *DiskonMasterService) ListApplicable(ctx context.Context, subtotal int64, t time.Time) ([]domain.DiskonMaster, error) {
	all, err := s.repo.List(ctx, repo.DiskonFilter{OnlyActive: true, AtTime: &t})
	if err != nil {
		return nil, err
	}
	out := make([]domain.DiskonMaster, 0, len(all))
	for _, d := range all {
		if d.IsApplicable(subtotal, t) {
			out = append(out, d)
		}
	}
	return out, nil
}

// Create master diskon. Konversi rupiah utuh → cents.
func (s *DiskonMasterService) Create(ctx context.Context, in dto.DiskonMasterInput) (*domain.DiskonMaster, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	d, err := buildDiskonFromInput(&in)
	if err != nil {
		return nil, err
	}
	// Cek dupe kode.
	if existing, err := s.repo.GetByKode(ctx, d.Kode); err == nil && existing != nil {
		return nil, domain.ErrDiskonKodeDuplikat
	} else if err != nil && !errors.Is(err, domain.ErrDiskonNotFound) {
		return nil, err
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create diskon: %w", err)
	}
	return d, nil
}

func (s *DiskonMasterService) Update(ctx context.Context, id int64, in dto.DiskonMasterInput) (*domain.DiskonMaster, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	d, err := buildDiskonFromInput(&in)
	if err != nil {
		return nil, err
	}
	d.ID = id
	if d.Kode != existing.Kode {
		if dupe, err := s.repo.GetByKode(ctx, d.Kode); err == nil && dupe != nil && dupe.ID != id {
			return nil, domain.ErrDiskonKodeDuplikat
		} else if err != nil && !errors.Is(err, domain.ErrDiskonNotFound) {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Toggle aktif/non-aktif.
func (s *DiskonMasterService) Toggle(ctx context.Context, id int64, active bool) error {
	return s.repo.SetActive(ctx, id, active)
}

// Delete - soft delete via is_active=false.
func (s *DiskonMasterService) Delete(ctx context.Context, id int64) error {
	return s.repo.SetActive(ctx, id, false)
}

func buildDiskonFromInput(in *dto.DiskonMasterInput) (*domain.DiskonMaster, error) {
	dari, err := time.Parse("2006-01-02", strings.TrimSpace(in.BerlakuDari))
	if err != nil {
		return nil, domain.ErrDiskonTanggalInvalid
	}
	var sampai *time.Time
	if s := strings.TrimSpace(in.BerlakuSampai); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, domain.ErrDiskonTanggalInvalid
		}
		sampai = &t
	}
	var maxC *int64
	if in.MaxDiskon > 0 {
		v := in.MaxDiskon * 100
		maxC = &v
	}
	return &domain.DiskonMaster{
		Kode:          strings.ToUpper(strings.TrimSpace(in.Kode)),
		Nama:          strings.TrimSpace(in.Nama),
		Tipe:          in.Tipe,
		Nilai:         in.Nilai,
		MinSubtotal:   in.MinSubtotal * 100,
		MaxDiskon:     maxC,
		BerlakuDari:   dari,
		BerlakuSampai: sampai,
		IsActive:      in.IsActive,
	}, nil
}

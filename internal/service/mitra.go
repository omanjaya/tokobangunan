package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// MitraService orchestrasi use case mitra (CRUD + list).
type MitraService struct {
	repo *repo.MitraRepo
}

// NewMitraService konstruktor.
func NewMitraService(r *repo.MitraRepo) *MitraService {
	return &MitraService{repo: r}
}

// CreateMitraInput input service-level (sudah dalam cents).
type CreateMitraInput struct {
	Kode            string
	Nama            string
	Alamat          string
	Kontak          string
	NPWP            string
	Tipe            string
	LimitKreditCent int64
	JatuhTempoHari  int
	GudangDefaultID int64 // 0 = NULL
	Catatan         string
	IsActive        bool
}

// UpdateMitraInput input update service-level.
type UpdateMitraInput struct {
	ID              int64
	Kode            string
	Nama            string
	Alamat          string
	Kontak          string
	NPWP            string
	Tipe            string
	LimitKreditCent int64
	JatuhTempoHari  int
	GudangDefaultID int64
	Catatan         string
	IsActive        bool
	Version         int64 // optimistic concurrency; 0 = skip
}

// Create validasi + insert mitra.
func (s *MitraService) Create(ctx context.Context, in CreateMitraInput) (*domain.Mitra, error) {
	m := buildMitra(in.Kode, in.Nama, in.Alamat, in.Kontak, in.NPWP, in.Tipe,
		in.LimitKreditCent, in.JatuhTempoHari, in.GudangDefaultID, in.Catatan, in.IsActive)
	m.ID = 0
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create mitra: %w", err)
	}
	return m, nil
}

// Update validasi + update mitra. Return mitra yang sudah ter-update.
func (s *MitraService) Update(ctx context.Context, in UpdateMitraInput) (*domain.Mitra, error) {
	m := buildMitra(in.Kode, in.Nama, in.Alamat, in.Kontak, in.NPWP, in.Tipe,
		in.LimitKreditCent, in.JatuhTempoHari, in.GudangDefaultID, in.Catatan, in.IsActive)
	m.ID = in.ID
	m.Version = in.Version
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, m); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		return nil, fmt.Errorf("update mitra: %w", err)
	}
	return m, nil
}

// List paginated.
func (s *MitraService) List(ctx context.Context, f repo.ListMitraFilter) (PageResult[domain.Mitra], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.Mitra]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Get satu mitra.
func (s *MitraService) Get(ctx context.Context, id int64) (*domain.Mitra, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete soft delete.
func (s *MitraService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDelete(ctx, id)
}

// Search autocomplete.
func (s *MitraService) Search(ctx context.Context, q string, limit int) ([]domain.Mitra, error) {
	return s.repo.Search(ctx, q, limit)
}

// buildMitra utility membentuk *Mitra dari raw input (handle string→*string).
func buildMitra(kode, nama, alamat, kontak, npwp, tipe string,
	limitCent int64, jatuhTempo int, gudangID int64, catatan string, isActive bool) *domain.Mitra {
	m := &domain.Mitra{
		Kode:           strings.TrimSpace(kode),
		Nama:           strings.TrimSpace(nama),
		Tipe:           strings.TrimSpace(tipe),
		LimitKredit:    limitCent,
		JatuhTempoHari: jatuhTempo,
		IsActive:       isActive,
	}
	if v := strings.TrimSpace(alamat); v != "" {
		m.Alamat = &v
	}
	if v := strings.TrimSpace(kontak); v != "" {
		m.Kontak = &v
	}
	if v := strings.TrimSpace(npwp); v != "" {
		m.NPWP = &v
	}
	if v := strings.TrimSpace(catatan); v != "" {
		m.Catatan = &v
	}
	if gudangID > 0 {
		m.GudangDefaultID = &gudangID
	}
	return m
}

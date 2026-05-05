package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// SupplierService orchestrasi use case supplier (CRUD + list).
type SupplierService struct {
	repo *repo.SupplierRepo
}

// NewSupplierService konstruktor.
func NewSupplierService(r *repo.SupplierRepo) *SupplierService {
	return &SupplierService{repo: r}
}

// CreateSupplierInput input service-level.
type CreateSupplierInput struct {
	Kode     string
	Nama     string
	Alamat   string
	Kontak   string
	Catatan  string
	IsActive bool
}

// UpdateSupplierInput input update service-level.
type UpdateSupplierInput struct {
	ID       int64
	Kode     string
	Nama     string
	Alamat   string
	Kontak   string
	Catatan  string
	IsActive bool
}

// Create validasi + insert supplier.
func (s *SupplierService) Create(ctx context.Context, in CreateSupplierInput) (*domain.Supplier, error) {
	sup := buildSupplier(in.Kode, in.Nama, in.Alamat, in.Kontak, in.Catatan, in.IsActive)
	if err := sup.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, sup); err != nil {
		return nil, fmt.Errorf("create supplier: %w", err)
	}
	return sup, nil
}

// Update validasi + update supplier.
func (s *SupplierService) Update(ctx context.Context, in UpdateSupplierInput) (*domain.Supplier, error) {
	sup := buildSupplier(in.Kode, in.Nama, in.Alamat, in.Kontak, in.Catatan, in.IsActive)
	sup.ID = in.ID
	if err := sup.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, sup); err != nil {
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	return sup, nil
}

// List paginated.
func (s *SupplierService) List(ctx context.Context, f repo.ListSupplierFilter) (PageResult[domain.Supplier], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.Supplier]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Get satu supplier.
func (s *SupplierService) Get(ctx context.Context, id int64) (*domain.Supplier, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete soft delete.
func (s *SupplierService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDelete(ctx, id)
}

func buildSupplier(kode, nama, alamat, kontak, catatan string, isActive bool) *domain.Supplier {
	sup := &domain.Supplier{
		Kode:     strings.TrimSpace(kode),
		Nama:     strings.TrimSpace(nama),
		IsActive: isActive,
	}
	if v := strings.TrimSpace(alamat); v != "" {
		sup.Alamat = &v
	}
	if v := strings.TrimSpace(kontak); v != "" {
		sup.Kontak = &v
	}
	if v := strings.TrimSpace(catatan); v != "" {
		sup.Catatan = &v
	}
	return sup
}

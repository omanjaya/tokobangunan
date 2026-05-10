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
	repo  *repo.SupplierRepo
	audit *AuditLogService // optional; nil-safe
}

// NewSupplierService konstruktor.
func NewSupplierService(r *repo.SupplierRepo) *SupplierService {
	return &SupplierService{repo: r}
}

// SetAudit attach AuditLogService (best-effort).
func (s *SupplierService) SetAudit(a *AuditLogService) { s.audit = a }

func (s *SupplierService) logAudit(ctx context.Context, aksi string, id int64, before, after any) {
	if s.audit == nil {
		return
	}
	var uid *int64
	if v := AuditUserFromContext(ctx); v > 0 {
		v2 := v
		uid = &v2
	}
	_ = s.audit.Record(ctx, RecordEntry{
		UserID: uid, Aksi: aksi, Tabel: "supplier", RecordID: id,
		Before: before, After: after,
	})
}

func supplierAuditPayload(sp *domain.Supplier) map[string]any {
	if sp == nil {
		return nil
	}
	return map[string]any{
		"id":        sp.ID,
		"kode":      sp.Kode,
		"nama":      sp.Nama,
		"alamat":    sp.Alamat,
		"kontak":    sp.Kontak,
		"is_active": sp.IsActive,
	}
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
	s.logAudit(ctx, "create", sup.ID, nil, supplierAuditPayload(sup))
	return sup, nil
}

// Update validasi + update supplier.
func (s *SupplierService) Update(ctx context.Context, in UpdateSupplierInput) (*domain.Supplier, error) {
	sup := buildSupplier(in.Kode, in.Nama, in.Alamat, in.Kontak, in.Catatan, in.IsActive)
	sup.ID = in.ID
	if err := sup.Validate(); err != nil {
		return nil, err
	}
	var beforeSnap any
	if old, errOld := s.repo.GetByID(ctx, in.ID); errOld == nil {
		beforeSnap = supplierAuditPayload(old)
	}
	if err := s.repo.Update(ctx, sup); err != nil {
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	s.logAudit(ctx, "update", sup.ID, beforeSnap, supplierAuditPayload(sup))
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
	var beforeSnap any
	if old, errOld := s.repo.GetByID(ctx, id); errOld == nil {
		beforeSnap = supplierAuditPayload(old)
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.logAudit(ctx, "delete", id, beforeSnap, nil)
	return nil
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

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

// ProdukService - use case untuk modul produk.
type ProdukService struct {
	produk *repo.ProdukRepo
	satuan *repo.SatuanRepo
}

func NewProdukService(p *repo.ProdukRepo, s *repo.SatuanRepo) *ProdukService {
	return &ProdukService{produk: p, satuan: s}
}

// List - bungkus repo + paging metadata.
func (s *ProdukService) List(ctx context.Context, f repo.ListProdukFilter) (PageResult[domain.Produk], error) {
	items, total, err := s.produk.List(ctx, f)
	if err != nil {
		return PageResult[domain.Produk]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Search untuk autocomplete produk picker.
func (s *ProdukService) Search(ctx context.Context, q string, limit int) ([]domain.Produk, error) {
	return s.produk.Search(ctx, q, limit)
}

// Get satu produk by ID.
func (s *ProdukService) Get(ctx context.Context, id int64) (*domain.Produk, error) {
	return s.produk.GetByID(ctx, id)
}

// Create - validate input + cek SKU unique + insert.
func (s *ProdukService) Create(ctx context.Context, in dto.ProdukCreateInput) (*domain.Produk, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}
	p := buildProdukFromInput(in.SKU, in.Nama, in.Kategori, in.SatuanKecilID,
		in.SatuanBesarID, in.FaktorKonversi, in.StokMinimum, in.IsActive)
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.assertSatuanExist(ctx, p.SatuanKecilID, p.SatuanBesarID); err != nil {
		return nil, err
	}
	if err := s.assertSKUUnique(ctx, p.SKU, 0); err != nil {
		return nil, err
	}
	if err := s.produk.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create produk: %w", err)
	}
	return p, nil
}

// Update - validate + cek SKU unique (exclude self) + update.
func (s *ProdukService) Update(ctx context.Context, id int64, in dto.ProdukUpdateInput) (*domain.Produk, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}
	existing, err := s.produk.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updated := buildProdukFromInput(in.SKU, in.Nama, in.Kategori, in.SatuanKecilID,
		in.SatuanBesarID, in.FaktorKonversi, in.StokMinimum, in.IsActive)
	updated.ID = existing.ID
	updated.Version = in.Version
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := s.assertSatuanExist(ctx, updated.SatuanKecilID, updated.SatuanBesarID); err != nil {
		return nil, err
	}
	if err := s.assertSKUUnique(ctx, updated.SKU, existing.ID); err != nil {
		return nil, err
	}
	if err := s.produk.Update(ctx, updated); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		return nil, fmt.Errorf("update produk: %w", err)
	}
	return updated, nil
}

// Delete soft-delete produk.
func (s *ProdukService) Delete(ctx context.Context, id int64) error {
	return s.produk.SoftDelete(ctx, id)
}

// SetFotoURL update kolom foto_url. Pass nil untuk clear.
func (s *ProdukService) SetFotoURL(ctx context.Context, id int64, url *string) error {
	return s.produk.UpdateFotoURL(ctx, id, url)
}

// ListKategori distinct kategori untuk filter dropdown.
func (s *ProdukService) ListKategori(ctx context.Context) ([]string, error) {
	return s.produk.ListKategori(ctx)
}

// ----- helpers ---------------------------------------------------------------

func buildProdukFromInput(sku, nama, kategori string, satKecil, satBesar int64,
	faktor, stokMin float64, active bool) *domain.Produk {
	p := &domain.Produk{
		SKU:            strings.TrimSpace(sku),
		Nama:           strings.TrimSpace(nama),
		SatuanKecilID:  satKecil,
		FaktorKonversi: faktor,
		StokMinimum:    stokMin,
		IsActive:       active,
	}
	if k := strings.TrimSpace(kategori); k != "" {
		p.Kategori = &k
	}
	if satBesar > 0 {
		p.SatuanBesarID = &satBesar
	}
	return p
}

func (s *ProdukService) assertSatuanExist(ctx context.Context, kecilID int64, besarID *int64) error {
	if _, err := s.satuan.GetByID(ctx, kecilID); err != nil {
		return fmt.Errorf("satuan kecil: %w", err)
	}
	if besarID != nil {
		if _, err := s.satuan.GetByID(ctx, *besarID); err != nil {
			return fmt.Errorf("satuan besar: %w", err)
		}
	}
	return nil
}

func (s *ProdukService) assertSKUUnique(ctx context.Context, sku string, excludeID int64) error {
	other, err := s.produk.GetBySKU(ctx, sku)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return nil
		}
		return err
	}
	if other.ID != excludeID {
		return domain.ErrSKUDuplikat
	}
	return nil
}

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// CashflowService - use case kas masuk/keluar.
type CashflowService struct {
	repo *repo.CashflowRepo
}

func NewCashflowService(r *repo.CashflowRepo) *CashflowService {
	return &CashflowService{repo: r}
}

// Create validate, generate nomor, persist.
func (s *CashflowService) Create(ctx context.Context, userID int64, in dto.CashflowCreateInput) (*domain.Cashflow, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}
	tgl, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	c := &domain.Cashflow{
		Tanggal:   tgl,
		GudangID:  in.GudangID,
		Tipe:      domain.CashflowTipe(in.Tipe),
		Kategori:  strings.TrimSpace(in.Kategori),
		Deskripsi: strings.TrimSpace(in.Deskripsi),
		Jumlah:    in.Jumlah * 100, // rupiah → cents
		Metode:    in.Metode,
		Referensi: strings.TrimSpace(in.Referensi),
		UserID:    userID,
		Catatan:   strings.TrimSpace(in.Catatan),
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	nomor, err := s.repo.NextNomor(ctx, c.Tipe, c.Tanggal)
	if err != nil {
		return nil, err
	}
	c.Nomor = nomor
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Get ambil 1 cashflow by id.
func (s *CashflowService) Get(ctx context.Context, id int64) (*domain.Cashflow, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete cashflow by id.
func (s *CashflowService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// List paginated.
func (s *CashflowService) List(ctx context.Context, f repo.ListCashflowFilter) (PageResult[domain.Cashflow], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.Cashflow]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Summary periode.
func (s *CashflowService) Summary(ctx context.Context, from, to time.Time, gudangID *int64) (domain.CashflowSummary, error) {
	return s.repo.SummaryPeriode(ctx, from, to, gudangID)
}

// KategoriBreakdown top kategori.
func (s *CashflowService) KategoriBreakdown(
	ctx context.Context, from, to time.Time, tipe domain.CashflowTipe, gudangID *int64, limit int,
) ([]domain.CashflowKategoriBreakdown, error) {
	return s.repo.KategoriBreakdown(ctx, from, to, tipe, gudangID, limit)
}

// DailyTrend tren harian.
func (s *CashflowService) DailyTrend(
	ctx context.Context, from, to time.Time, gudangID *int64,
) ([]domain.CashflowDailyPoint, error) {
	return s.repo.DailyTrend(ctx, from, to, gudangID)
}

// ListKategori master kategori; pass tipe="" untuk semua.
func (s *CashflowService) ListKategori(ctx context.Context, tipe domain.CashflowTipe) ([]domain.CashflowKategori, error) {
	return s.repo.ListKategori(ctx, tipe)
}

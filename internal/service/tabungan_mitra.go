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

// TabunganService orchestrasi setor/tarik + history tabungan mitra.
type TabunganService struct {
	tabunganRepo *repo.TabunganMitraRepo
	mitraRepo    *repo.MitraRepo
}

// NewTabunganService konstruktor.
func NewTabunganService(tabunganRepo *repo.TabunganMitraRepo, mitraRepo *repo.MitraRepo) *TabunganService {
	return &TabunganService{tabunganRepo: tabunganRepo, mitraRepo: mitraRepo}
}

// Setor catat setoran (debit) ke tabungan mitra.
func (s *TabunganService) Setor(ctx context.Context, in dto.TabunganSetorInput, userID int64) (*domain.TabunganMitra, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.mitraRepo.GetByID(ctx, in.MitraID); err != nil {
		return nil, err
	}
	t := &domain.TabunganMitra{
		MitraID: in.MitraID,
		Tanggal: tanggal,
		Debit:   in.Jumlah * 100,
		Kredit:  0,
		Catatan: strings.TrimSpace(in.Catatan),
		UserID:  userID,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.tabunganRepo.Insert(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Tarik catat penarikan (kredit). Cek saldo cukup di repo (FOR UPDATE).
func (s *TabunganService) Tarik(ctx context.Context, in dto.TabunganTarikInput, userID int64) (*domain.TabunganMitra, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.mitraRepo.GetByID(ctx, in.MitraID); err != nil {
		return nil, err
	}
	t := &domain.TabunganMitra{
		MitraID: in.MitraID,
		Tanggal: tanggal,
		Debit:   0,
		Kredit:  in.Jumlah * 100,
		Catatan: strings.TrimSpace(in.Catatan),
		UserID:  userID,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.tabunganRepo.Insert(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Saldo saldo terkini.
func (s *TabunganService) Saldo(ctx context.Context, mitraID int64) (int64, error) {
	return s.tabunganRepo.GetSaldo(ctx, mitraID)
}

// History paginated ledger.
func (s *TabunganService) History(ctx context.Context, mitraID int64, f repo.ListTabunganFilter) (PageResult[domain.TabunganMitra], error) {
	items, total, err := s.tabunganRepo.ListByMitra(ctx, mitraID, f)
	if err != nil {
		return PageResult[domain.TabunganMitra]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

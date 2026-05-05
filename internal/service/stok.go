package service

import (
	"context"

	"github.com/omanjaya/tokobangunan/internal/repo"
)

// StokService - use case query stok.
type StokService struct {
	stok   *repo.StokRepo
	produk *repo.ProdukRepo
	gudang *repo.GudangRepo
}

func NewStokService(s *repo.StokRepo, p *repo.ProdukRepo, g *repo.GudangRepo) *StokService {
	return &StokService{stok: s, produk: p, gudang: g}
}

// ListByGudang - listing stok di satu gudang.
func (s *StokService) ListByGudang(ctx context.Context, gudangID int64, f repo.ListStokFilter) (PageResult[repo.StokDetail], error) {
	items, total, err := s.stok.ListByGudang(ctx, gudangID, f)
	if err != nil {
		return PageResult[repo.StokDetail]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// ListLowStock - produk yang qty <= stok_minimum.
func (s *StokService) ListLowStock(ctx context.Context, gudangID *int64) ([]repo.StokDetail, error) {
	return s.stok.ListLowStock(ctx, gudangID)
}

// Snapshot - posisi stok 1 produk di semua gudang.
func (s *StokService) Snapshot(ctx context.Context, produkID int64) ([]repo.StokDetail, error) {
	return s.stok.ListAllByProduk(ctx, produkID)
}

// MultiGudangSummary - peta produk_id -> gudang_id -> qty.
func (s *StokService) MultiGudangSummary(ctx context.Context, produkIDs []int64) (map[int64]map[int64]float64, error) {
	return s.stok.MultiGudangSummary(ctx, produkIDs)
}

// Get - qty 1 produk di 1 gudang.
func (s *StokService) Get(ctx context.Context, gudangID, produkID int64) (float64, error) {
	return s.stok.Get(ctx, gudangID, produkID)
}

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// HargaService - kelola history harga produk.
type HargaService struct {
	repo   *repo.HargaRepo
	produk *repo.ProdukRepo
}

func NewHargaService(h *repo.HargaRepo, p *repo.ProdukRepo) *HargaService {
	return &HargaService{repo: h, produk: p}
}

// SetHarga insert baris baru ke history (dipakai sebagai harga aktif baru).
// HargaJual diterima dalam Rupiah utuh, disimpan dalam cents.
func (s *HargaService) SetHarga(ctx context.Context, produkID int64, in dto.HargaSetInput) (*domain.HargaProduk, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}
	if _, err := s.produk.GetByID(ctx, produkID); err != nil {
		return nil, err
	}
	tanggal, err := time.Parse("2006-01-02", in.BerlakuDari)
	if err != nil {
		return nil, domain.ErrHargaTanggalInvalid
	}
	h := &domain.HargaProduk{
		ProdukID:    produkID,
		Tipe:        in.Tipe,
		HargaJual:   in.HargaJual * 100, // Rupiah → cents
		BerlakuDari: tanggal,
	}
	if in.GudangID > 0 {
		gid := in.GudangID
		h.GudangID = &gid
	}
	if err := h.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, h); err != nil {
		return nil, fmt.Errorf("set harga: %w", err)
	}
	return h, nil
}

// GetAktif - delegasi ke repo.
func (s *HargaService) GetAktif(ctx context.Context, produkID int64, gudangID *int64, tipe string) (*domain.HargaProduk, error) {
	if !domain.IsValidTipeHarga(tipe) {
		return nil, domain.ErrHargaTipeInvalid
	}
	return s.repo.GetAktif(ctx, produkID, gudangID, tipe)
}

// ListByProduk seluruh history harga.
func (s *HargaService) ListByProduk(ctx context.Context, produkID int64) ([]domain.HargaProduk, error) {
	return s.repo.ListByProduk(ctx, produkID)
}

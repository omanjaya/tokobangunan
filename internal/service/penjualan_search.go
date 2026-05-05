package service

import (
	"context"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// Search melakukan autocomplete penjualan berdasarkan nomor kwitansi.
// Wrapper tipis untuk PenjualanRepo.SearchByNomor agar handler tidak perlu
// menyentuh repo langsung dan konsisten dengan ProdukService.Search /
// MitraService.Search.
func (s *PenjualanService) Search(ctx context.Context, q string, limit int) ([]domain.Penjualan, error) {
	return s.penjualan.SearchByNomor(ctx, q, limit)
}

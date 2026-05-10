package service

import (
	"context"

	"github.com/omanjaya/tokobangunan/internal/repo"
)

// ForecastService - use case inventory forecasting + reorder point.
type ForecastService struct {
	forecast *repo.ForecastRepo
}

func NewForecastService(f *repo.ForecastRepo) *ForecastService {
	return &ForecastService{forecast: f}
}

// Velocity - list produk yang stoknya di bawah reorder point.
func (s *ForecastService) Velocity(ctx context.Context, lookbackDays int, gudangID *int64) ([]repo.ProdukVelocity, error) {
	return s.forecast.Velocity(ctx, lookbackDays, gudangID)
}

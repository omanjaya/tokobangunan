package service

import (
	"context"
	"fmt"
	"time"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// DashboardData - bundle untuk halaman dashboard.
type DashboardData struct {
	KPI            *repo.DashboardKPI
	SalesLast30    []repo.SalesPerDay
	TopMitra       []repo.TopMitra
	StokKritis     []repo.StokKritisRow
	RecentTrx      []repo.LaporanPenjualanRow
	RecentPembayaran []repo.RecentPembayaranRow
	RecentMutasi   []repo.RecentMutasiRow
	ScopeNama      string
}

// LaporanPenjualanFilter - parameter laporan penjualan list.
type LaporanPenjualanFilter struct {
	From     time.Time
	To       time.Time
	GudangID *int64
	MitraID  *int64
	Page     int
	PerPage  int
}

// LaporanMutasiFilter - parameter laporan mutasi.
type LaporanMutasiFilter struct {
	From     time.Time
	To       time.Time
	GudangID *int64
}

type LaporanService struct {
	laporan *repo.LaporanRepo
	gudang  *repo.GudangRepo
}

func NewLaporanService(lr *repo.LaporanRepo, gr *repo.GudangRepo) *LaporanService {
	return &LaporanService{laporan: lr, gudang: gr}
}

// Dashboard - kumpulkan KPI + grafik + top mitra + stok kritis + recent.
// Owner/admin: semua gudang. Kasir/staff dengan gudang_id: hanya gudangnya.
func (s *LaporanService) Dashboard(ctx context.Context, user *auth.User) (*DashboardData, error) {
	var gudangID *int64
	scopeNama := "Semua Cabang"
	if user != nil && user.Role != "owner" && user.Role != "admin" && user.GudangID != nil {
		gid := *user.GudangID
		gudangID = &gid
		if g, err := s.gudang.GetByID(ctx, gid); err == nil {
			scopeNama = g.Nama
		}
	}

	kpi, err := s.laporan.GetDashboardKPI(ctx, gudangID)
	if err != nil {
		return nil, err
	}
	sales, err := s.laporan.SalesLast30Days(ctx, gudangID)
	if err != nil {
		return nil, err
	}
	to := time.Now()
	from := to.AddDate(0, 0, -29)
	topMitra, err := s.laporan.TopMitraPeriod(ctx, from, to, 5)
	if err != nil {
		return nil, err
	}
	stokKritis, err := s.laporan.StokKritisAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(stokKritis) > 10 {
		stokKritis = stokKritis[:10]
	}
	recent, err := s.laporan.RecentTransaksi(ctx, gudangID, 10)
	if err != nil {
		return nil, err
	}
	// Recent payments & mutations — best effort (jangan gagalkan dashboard).
	recentBayar, _ := s.laporan.RecentPembayaran(ctx, 5)
	recentMutasi, _ := s.laporan.RecentMutasi(ctx, 5)

	return &DashboardData{
		KPI:              kpi,
		SalesLast30:      sales,
		TopMitra:         topMitra,
		StokKritis:       stokKritis,
		RecentTrx:        recent,
		RecentPembayaran: recentBayar,
		RecentMutasi:     recentMutasi,
		ScopeNama:        scopeNama,
	}, nil
}

// LR - laba rugi per gudang. Default range = 30 hari terakhir kalau zero.
func (s *LaporanService) LR(ctx context.Context, from, to time.Time) ([]repo.LaporanLR, error) {
	from, to = normalizeRange(from, to)
	if to.Before(from) {
		return nil, fmt.Errorf("rentang tanggal tidak valid")
	}
	return s.laporan.LaporanLRPerGudang(ctx, from, to)
}

// Penjualan - laporan list penjualan dengan pagination.
func (s *LaporanService) Penjualan(
	ctx context.Context, f LaporanPenjualanFilter,
) ([]repo.LaporanPenjualanRow, int, error) {
	f.From, f.To = normalizeRange(f.From, f.To)
	if f.To.Before(f.From) {
		return nil, 0, fmt.Errorf("rentang tanggal tidak valid")
	}
	return s.laporan.LaporanPenjualan(ctx, f.From, f.To, f.GudangID, f.MitraID, f.Page, f.PerPage)
}

// Mutasi - laporan mutasi periode.
func (s *LaporanService) Mutasi(ctx context.Context, f LaporanMutasiFilter) ([]repo.LaporanMutasiRow, error) {
	f.From, f.To = normalizeRange(f.From, f.To)
	if f.To.Before(f.From) {
		return nil, fmt.Errorf("rentang tanggal tidak valid")
	}
	return s.laporan.LaporanMutasi(ctx, f.From, f.To, f.GudangID)
}

// StokKritis - passthrough.
func (s *LaporanService) StokKritis(ctx context.Context) ([]repo.StokKritisRow, error) {
	return s.laporan.StokKritisAll(ctx)
}

// TopProduk - top produk periode.
func (s *LaporanService) TopProduk(
	ctx context.Context, from, to time.Time, limit int,
) ([]repo.TopProduk, error) {
	from, to = normalizeRange(from, to)
	if to.Before(from) {
		return nil, fmt.Errorf("rentang tanggal tidak valid")
	}
	return s.laporan.TopProdukPeriod(ctx, from, to, limit)
}

// normalizeRange - kalau zero, default 30 hari terakhir.
func normalizeRange(from, to time.Time) (time.Time, time.Time) {
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -29)
	}
	// Truncate ke tanggal saja.
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
	return from, to
}

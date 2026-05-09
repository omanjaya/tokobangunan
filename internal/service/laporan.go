package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

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

	to := time.Now()
	from := to.AddDate(0, 0, -29)

	// Parallelize 7 aggregate queries — pool conn (default 20) cukup.
	// Total latency = max(query) bukan sum, ~5x speedup di dashboard cold.
	var (
		kpi          *repo.DashboardKPI
		sales        []repo.SalesPerDay
		topMitra     []repo.TopMitra
		stokKritis   []repo.StokKritisRow
		recent       []repo.LaporanPenjualanRow
		recentBayar  []repo.RecentPembayaranRow
		recentMutasi []repo.RecentMutasiRow
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var e error
		kpi, e = s.laporan.GetDashboardKPI(gctx, gudangID)
		return e
	})
	g.Go(func() error {
		var e error
		sales, e = s.laporan.SalesLast30Days(gctx, gudangID)
		return e
	})
	g.Go(func() error {
		var e error
		topMitra, e = s.laporan.TopMitraPeriod(gctx, from, to, 5)
		return e
	})
	g.Go(func() error {
		var e error
		stokKritis, e = s.laporan.StokKritisAll(gctx)
		return e
	})
	g.Go(func() error {
		var e error
		recent, e = s.laporan.RecentTransaksi(gctx, gudangID, 10)
		return e
	})
	// Best-effort — jangan gagalkan dashboard kalau dua ini error.
	g.Go(func() error {
		recentBayar, _ = s.laporan.RecentPembayaran(gctx, 5)
		return nil
	})
	g.Go(func() error {
		recentMutasi, _ = s.laporan.RecentMutasi(gctx, 5)
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if len(stokKritis) > 10 {
		stokKritis = stokKritis[:10]
	}

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

// DashboardScope - resolve gudang scope berdasar role user.
func (s *LaporanService) DashboardScope(ctx context.Context, user *auth.User) *int64 {
	if user != nil && user.Role != "owner" && user.Role != "admin" && user.GudangID != nil {
		gid := *user.GudangID
		return &gid
	}
	return nil
}

// DashboardAboveFold - hanya KPI + sales 30 hari + top mitra (above-fold).
// Dipakai initial page load supaya FCP/LCP turun (skip 4 query bawah).
func (s *LaporanService) DashboardAboveFold(ctx context.Context, user *auth.User) (*DashboardData, error) {
	gudangID := s.DashboardScope(ctx, user)
	scopeNama := "Semua Cabang"
	if gudangID != nil {
		if g, err := s.gudang.GetByID(ctx, *gudangID); err == nil {
			scopeNama = g.Nama
		}
	}

	to := time.Now()
	from := to.AddDate(0, 0, -29)

	var (
		kpi      *repo.DashboardKPI
		sales    []repo.SalesPerDay
		topMitra []repo.TopMitra
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var e error
		kpi, e = s.laporan.GetDashboardKPI(gctx, gudangID)
		return e
	})
	g.Go(func() error {
		var e error
		sales, e = s.laporan.SalesLast30Days(gctx, gudangID)
		return e
	})
	g.Go(func() error {
		var e error
		topMitra, e = s.laporan.TopMitraPeriod(gctx, from, to, 5)
		return e
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &DashboardData{
		KPI:         kpi,
		SalesLast30: sales,
		TopMitra:    topMitra,
		ScopeNama:   scopeNama,
	}, nil
}

// DashboardStokKritis - section bawah, untuk lazy htmx fetch.
func (s *LaporanService) DashboardStokKritis(ctx context.Context, user *auth.User) ([]repo.StokKritisRow, error) {
	rows, err := s.laporan.StokKritisAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) > 10 {
		rows = rows[:10]
	}
	return rows, nil
}

// DashboardRecentTrx - section bawah.
func (s *LaporanService) DashboardRecentTrx(ctx context.Context, user *auth.User) ([]repo.LaporanPenjualanRow, error) {
	gudangID := s.DashboardScope(ctx, user)
	return s.laporan.RecentTransaksi(ctx, gudangID, 10)
}

// DashboardRecentPembayaran - section bawah.
func (s *LaporanService) DashboardRecentPembayaran(ctx context.Context) ([]repo.RecentPembayaranRow, error) {
	rows, _ := s.laporan.RecentPembayaran(ctx, 5)
	return rows, nil
}

// DashboardRecentMutasi - section bawah.
func (s *LaporanService) DashboardRecentMutasi(ctx context.Context) ([]repo.RecentMutasiRow, error) {
	rows, _ := s.laporan.RecentMutasi(ctx, 5)
	return rows, nil
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

package service

import (
	"context"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// PiutangService orchestrasi query piutang per mitra.
type PiutangService struct {
	piutangRepo *repo.PiutangRepo
	mitraRepo   *repo.MitraRepo
}

// NewPiutangService konstruktor.
func NewPiutangService(piutangRepo *repo.PiutangRepo, mitraRepo *repo.MitraRepo) *PiutangService {
	return &PiutangService{piutangRepo: piutangRepo, mitraRepo: mitraRepo}
}

// Summary list mitra dengan piutang outstanding.
func (s *PiutangService) Summary(ctx context.Context, f repo.ListPiutangFilter) (PageResult[domain.PiutangSummary], error) {
	items, total, err := s.piutangRepo.SummaryAll(ctx, f)
	if err != nil {
		return PageResult[domain.PiutangSummary]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// MitraDetail detail piutang satu mitra: profil + summary + list invoice belum lunas.
func (s *PiutangService) MitraDetail(ctx context.Context, mitraID int64) (*domain.Mitra, *domain.PiutangSummary, []domain.PiutangInvoice, error) {
	m, err := s.mitraRepo.GetByID(ctx, mitraID)
	if err != nil {
		return nil, nil, nil, err
	}
	invs, _, err := s.piutangRepo.InvoicesByMitra(ctx, mitraID, repo.ListInvoiceFilter{Page: 1, PerPage: 200})
	if err != nil {
		return m, nil, nil, err
	}

	// Build summary dari invoice list.
	var sum domain.PiutangSummary
	sum.MitraID = m.ID
	sum.MitraNama = m.Nama
	sum.MitraKode = m.Kode
	sum.JumlahInvoice = len(invs)
	maxOverdue := 0
	for _, inv := range invs {
		sum.TotalPenjualan += inv.Total
		sum.TotalDibayar += inv.Dibayar
		sum.Outstanding += inv.Outstanding
		if sum.InvoiceTertua == nil || inv.Tanggal.Before(*sum.InvoiceTertua) {
			tg := inv.Tanggal
			sum.InvoiceTertua = &tg
		}
		if inv.HariOverdue > maxOverdue {
			maxOverdue = inv.HariOverdue
		}
	}
	sum.Aging = domain.AgingFromDays(maxOverdue)
	return m, &sum, invs, nil
}

// AgingBuckets agregat per bucket aging untuk dashboard summary.
func (s *PiutangService) AgingBuckets(ctx context.Context) (map[domain.PiutangAging]int64, error) {
	return s.piutangRepo.AgingBuckets(ctx)
}

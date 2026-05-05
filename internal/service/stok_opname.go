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

// StokOpnameService orchestrasi use case stok opname.
type StokOpnameService struct {
	opnameRepo *repo.StokOpnameRepo
	gudangRepo *repo.GudangRepo
}

// NewStokOpnameService konstruktor.
func NewStokOpnameService(opnameRepo *repo.StokOpnameRepo, gudangRepo *repo.GudangRepo) *StokOpnameService {
	return &StokOpnameService{opnameRepo: opnameRepo, gudangRepo: gudangRepo}
}

// Create generate nomor + insert header status=draft + pre-fill items dari snapshot stok.
func (s *StokOpnameService) Create(ctx context.Context, in dto.StokOpnameCreateInput, userID int64) (*domain.StokOpname, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.gudangRepo.GetByID(ctx, in.GudangID); err != nil {
		return nil, err
	}

	nomor, err := s.opnameRepo.NextNomor(ctx, tanggal)
	if err != nil {
		return nil, err
	}

	// Pre-fill items dari stok saat ini di gudang.
	snap, err := s.opnameRepo.LoadCurrentStokForGudang(ctx, in.GudangID)
	if err != nil {
		// Tabel stok mungkin belum ada (Fase 3 belum selesai). Tetap lanjut header-only.
		snap = nil
	}
	items := make([]domain.StokOpnameItem, 0, len(snap))
	for _, sn := range snap {
		items = append(items, domain.StokOpnameItem{
			ProdukID:   sn.ProdukID,
			ProdukNama: sn.ProdukNama,
			QtySistem:  sn.QtySistem,
			QtyFisik:   sn.QtySistem, // default = sistem (selisih = 0)
		})
	}

	o := &domain.StokOpname{
		Nomor:    nomor,
		GudangID: in.GudangID,
		Tanggal:  tanggal,
		UserID:   userID,
		Status:   domain.OpnameDraft,
		Catatan:  strings.TrimSpace(in.Catatan),
		Items:    items,
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if err := s.opnameRepo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// Get load opname + items.
func (s *StokOpnameService) Get(ctx context.Context, id int64) (*domain.StokOpname, error) {
	o, err := s.opnameRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.opnameRepo.LoadItems(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// List paginated.
func (s *StokOpnameService) List(ctx context.Context, f repo.ListStokOpnameFilter) (PageResult[domain.StokOpname], error) {
	items, total, err := s.opnameRepo.List(ctx, f)
	if err != nil {
		return PageResult[domain.StokOpname]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// UpdateItem set qty_fisik & keterangan untuk satu produk. Hanya draft.
func (s *StokOpnameService) UpdateItem(ctx context.Context, opnameID, produkID int64, qtyFisik float64, keterangan string) error {
	if qtyFisik < 0 {
		return domain.ErrOpnameQtyFisikInvalid
	}
	o, err := s.opnameRepo.GetByID(ctx, opnameID)
	if err != nil {
		return err
	}
	if o.Status != domain.OpnameDraft {
		return domain.ErrOpnameTransitionInvalid
	}
	if err := s.opnameRepo.LoadItems(ctx, o); err != nil {
		return err
	}
	var qtySistem float64
	found := false
	for _, it := range o.Items {
		if it.ProdukID == produkID {
			qtySistem = it.QtySistem
			found = true
			break
		}
	}
	if !found {
		return domain.ErrOpnameItemTidakDitemukan
	}
	return s.opnameRepo.UpsertItem(ctx, opnameID, produkID, qtySistem, qtyFisik, strings.TrimSpace(keterangan))
}

// Submit transisi draft → selesai.
func (s *StokOpnameService) Submit(ctx context.Context, id int64) error {
	o, err := s.opnameRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !o.Status.CanTransitionTo(domain.OpnameSelesai) {
		return domain.ErrOpnameTransitionInvalid
	}
	return s.opnameRepo.UpdateStatus(ctx, id, domain.OpnameSelesai)
}

// Approve transisi selesai → approved (trigger DB akan apply qty_fisik ke stok).
func (s *StokOpnameService) Approve(ctx context.Context, id int64, userID int64) error {
	o, err := s.opnameRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !o.Status.CanTransitionTo(domain.OpnameApproved) {
		return domain.ErrOpnameTransitionInvalid
	}
	_ = userID // reserved untuk audit trail
	return s.opnameRepo.UpdateStatus(ctx, id, domain.OpnameApproved)
}

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// MutasiService - use case mutasi antar gudang.
type MutasiService struct {
	mutasi *repo.MutasiRepo
	stok   *repo.StokRepo
	produk *repo.ProdukRepo
	gudang *repo.GudangRepo
	satuan *repo.SatuanRepo
}

func NewMutasiService(m *repo.MutasiRepo, s *repo.StokRepo, p *repo.ProdukRepo,
	g *repo.GudangRepo, sa *repo.SatuanRepo) *MutasiService {
	return &MutasiService{mutasi: m, stok: s, produk: p, gudang: g, satuan: sa}
}

// Create - validate input, generate nomor, snapshot produk/satuan, insert.
// Bila SubmitNow=true, lanjut panggil Submit setelah create.
// parseOrNewUUID dipakai dari service/penjualan.go (paket sama).
func (s *MutasiService) Create(ctx context.Context, in dto.MutasiCreateInput, userID int64) (*domain.MutasiGudang, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}

	// Idempotency check via client_uuid.
	cuid, err := parseOrNewUUID(in.ClientUUID)
	if err != nil {
		return nil, err
	}
	if existing, err := s.mutasi.GetByClientUUID(ctx, cuid); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrMutasiNotFound) {
		return nil, err
	}

	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}

	// Validasi gudang.
	if _, err := s.gudang.GetByID(ctx, in.GudangAsalID); err != nil {
		return nil, fmt.Errorf("gudang asal: %w", err)
	}
	if _, err := s.gudang.GetByID(ctx, in.GudangTujuanID); err != nil {
		return nil, fmt.Errorf("gudang tujuan: %w", err)
	}

	// Build items (snapshot nama produk + kode satuan + hitung qty_konversi).
	items := make([]domain.MutasiItem, 0, len(in.Items))
	for _, it := range in.Items {
		p, err := s.produk.GetByID(ctx, it.ProdukID)
		if err != nil {
			return nil, fmt.Errorf("produk #%d: %w", it.ProdukID, err)
		}
		sat, err := s.satuan.GetByID(ctx, it.SatuanID)
		if err != nil {
			return nil, fmt.Errorf("satuan #%d: %w", it.SatuanID, err)
		}
		// Hitung qty dalam satuan_kecil. Bila pakai satuan_besar, kalikan faktor.
		qtyKonversi := it.Qty
		if p.SatuanBesarID != nil && *p.SatuanBesarID == it.SatuanID {
			qtyKonversi = it.Qty * p.FaktorKonversi
		}
		items = append(items, domain.MutasiItem{
			ProdukID:      p.ID,
			ProdukNama:    p.Nama,
			Qty:           it.Qty,
			SatuanID:      sat.ID,
			SatuanKode:    sat.Kode,
			QtyKonversi:   qtyKonversi,
			HargaInternal: it.HargaInternal,
			Catatan:       strings.TrimSpace(it.Catatan),
		})
	}

	nomor, err := s.mutasi.NextNomor(ctx, tanggal)
	if err != nil {
		return nil, err
	}

	m := &domain.MutasiGudang{
		NomorMutasi:    nomor,
		Tanggal:        tanggal,
		GudangAsalID:   in.GudangAsalID,
		GudangTujuanID: in.GudangTujuanID,
		Status:         domain.StatusDraft,
		UserPengirimID: &userID,
		Catatan:        strings.TrimSpace(in.Catatan),
		ClientUUID:     cuid,
		Items:          items,
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}

	if err := s.mutasi.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create mutasi: %w", err)
	}

	// Optional auto-submit.
	if in.SubmitNow {
		if err := s.Submit(ctx, m.ID, userID); err != nil {
			return nil, err
		}
		updated, err := s.Get(ctx, m.ID)
		if err == nil {
			return updated, nil
		}
	}
	return m, nil
}

// Submit - draft -> dikirim. Cek stok cukup di gudang_asal sebelum transisi.
// Trigger DB akan kurangi stok.
func (s *MutasiService) Submit(ctx context.Context, id, userID int64) error {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !m.Status.CanTransitionTo(domain.StatusDikirim) {
		return domain.ErrTransisiInvalid
	}
	if err := s.mutasi.LoadItems(ctx, m); err != nil {
		return err
	}
	for _, it := range m.Items {
		qty, err := s.stok.Get(ctx, m.GudangAsalID, it.ProdukID)
		if err != nil {
			return err
		}
		if qty < it.QtyKonversi {
			return fmt.Errorf("%w: %s (tersedia %.4f, butuh %.4f)",
				domain.ErrStokTidakCukup, it.ProdukNama, qty, it.QtyKonversi)
		}
	}
	return s.mutasi.UpdateStatus(ctx, id, domain.StatusDraft, domain.StatusDikirim, userID)
}

// Receive - dikirim -> diterima. Trigger DB akan tambah stok di gudang_tujuan.
func (s *MutasiService) Receive(ctx context.Context, id, userID int64) error {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !m.Status.CanTransitionTo(domain.StatusDiterima) {
		return domain.ErrTransisiInvalid
	}
	return s.mutasi.UpdateStatus(ctx, id, domain.StatusDikirim, domain.StatusDiterima, userID)
}

// Cancel - cancel mutasi.
//   - Dari draft: cukup ubah status (tidak menyentuh stok).
//   - Dari dikirim: stok di gudang_asal di-revert oleh trigger DB
//     (apply_mutasi_stok pada migration 0024).
func (s *MutasiService) Cancel(ctx context.Context, id, userID int64) error {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !m.Status.CanTransitionTo(domain.StatusDibatalkan) {
		return domain.ErrTransisiInvalid
	}
	return s.mutasi.UpdateStatus(ctx, id, m.Status, domain.StatusDibatalkan, userID)
}

// Get - mutasi + items.
func (s *MutasiService) Get(ctx context.Context, id int64) (*domain.MutasiGudang, error) {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.mutasi.LoadItems(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// List - bungkus repo + paging metadata.
func (s *MutasiService) List(ctx context.Context, f repo.ListMutasiFilter) (PageResult[domain.MutasiGudang], error) {
	items, total, err := s.mutasi.List(ctx, f)
	if err != nil {
		return PageResult[domain.MutasiGudang]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

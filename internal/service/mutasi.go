package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// MutasiService - use case mutasi antar gudang.
type MutasiService struct {
	mutasi *repo.MutasiRepo
	stok   *repo.StokRepo
	produk *repo.ProdukRepo
	gudang *repo.GudangRepo
	satuan *repo.SatuanRepo
	audit  *AuditLogService // optional; nil-safe
}

func NewMutasiService(m *repo.MutasiRepo, s *repo.StokRepo, p *repo.ProdukRepo,
	g *repo.GudangRepo, sa *repo.SatuanRepo) *MutasiService {
	return &MutasiService{mutasi: m, stok: s, produk: p, gudang: g, satuan: sa}
}

// SetAudit attach AuditLogService (best-effort).
func (s *MutasiService) SetAudit(a *AuditLogService) { s.audit = a }

func (s *MutasiService) logAudit(ctx context.Context, userID int64, aksi string, id int64, before, after any) {
	if s.audit == nil {
		return
	}
	uid := userID
	var uidp *int64
	if uid > 0 {
		uidp = &uid
	}
	_ = s.audit.Record(ctx, RecordEntry{
		UserID: uidp, Aksi: aksi, Tabel: "mutasi_gudang", RecordID: id,
		Before: before, After: after,
	})
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
	s.logAudit(ctx, userID, "create", m.ID, nil, map[string]any{
		"nomor_mutasi":     m.NomorMutasi,
		"tanggal":          m.Tanggal,
		"gudang_asal_id":   m.GudangAsalID,
		"gudang_tujuan_id": m.GudangTujuanID,
		"items_count":      len(m.Items),
		"status":           string(m.Status),
	})

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
// Trigger DB akan kurangi stok. Wrap di tx dengan FOR UPDATE pada stok asal
// supaya dua submit paralel tidak balapan.
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
	err = pgx.BeginFunc(ctx, s.mutasi.Pool(), func(tx pgx.Tx) error {
		// Lock + cek stok cukup di gudang_asal sebelum trigger melakukan update.
		seen := make(map[int64]struct{}, len(m.Items))
		need := make(map[int64]float64, len(m.Items))
		for _, it := range m.Items {
			need[it.ProdukID] += it.QtyKonversi
		}
		for produkID := range need {
			if _, ok := seen[produkID]; ok {
				continue
			}
			seen[produkID] = struct{}{}
			if err := s.mutasi.LockStokRow(ctx, tx, m.GudangAsalID, produkID); err != nil {
				return err
			}
		}
		for produkID, qtyNeed := range need {
			var current float64
			if err := tx.QueryRow(ctx,
				`SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2`,
				m.GudangAsalID, produkID,
			).Scan(&current); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					current = 0
				} else {
					return err
				}
			}
			if current < qtyNeed {
				return fmt.Errorf("%w: produk %d (tersedia %s, butuh %s)",
					domain.ErrStokTidakCukup, produkID, format.Qty(current), format.Qty(qtyNeed))
			}
		}
		return s.mutasi.UpdateStatusInTx(ctx, tx, id, domain.StatusDraft, domain.StatusDikirim, userID)
	})
	if err != nil {
		return err
	}
	s.logAudit(ctx, userID, "submit", id, map[string]any{"status": "draft"}, map[string]any{"status": "dikirim"})
	return nil
}

// Receive - dikirim -> diterima. Trigger DB akan tambah stok di gudang_tujuan.
// Wrap di tx + lock stok tujuan supaya konsisten dengan Submit.
func (s *MutasiService) Receive(ctx context.Context, id, userID int64) error {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !m.Status.CanTransitionTo(domain.StatusDiterima) {
		return domain.ErrTransisiInvalid
	}
	if err := s.mutasi.LoadItems(ctx, m); err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, s.mutasi.Pool(), func(tx pgx.Tx) error {
		seen := make(map[int64]struct{}, len(m.Items))
		for _, it := range m.Items {
			if _, ok := seen[it.ProdukID]; ok {
				continue
			}
			seen[it.ProdukID] = struct{}{}
			if err := s.mutasi.LockStokRow(ctx, tx, m.GudangTujuanID, it.ProdukID); err != nil {
				return err
			}
		}
		return s.mutasi.UpdateStatusInTx(ctx, tx, id, domain.StatusDikirim, domain.StatusDiterima, userID)
	})
	if err != nil {
		return err
	}
	s.logAudit(ctx, userID, "approve", id, map[string]any{"status": "dikirim"}, map[string]any{"status": "diterima"})
	return nil
}

// Cancel - cancel mutasi.
//   - Dari draft: cukup ubah status (tidak menyentuh stok).
//   - Dari dikirim: stok di gudang_asal di-revert oleh trigger DB
//     (apply_mutasi_stok pada migration 0024).
//
// Wrap di tx + lock stok asal kalau status sebelumnya dikirim.
func (s *MutasiService) Cancel(ctx context.Context, id, userID int64) error {
	m, err := s.mutasi.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !m.Status.CanTransitionTo(domain.StatusDibatalkan) {
		return domain.ErrTransisiInvalid
	}
	prev := m.Status
	if prev == domain.StatusDikirim {
		if err := s.mutasi.LoadItems(ctx, m); err != nil {
			return err
		}
	}
	err = pgx.BeginFunc(ctx, s.mutasi.Pool(), func(tx pgx.Tx) error {
		if prev == domain.StatusDikirim {
			seen := make(map[int64]struct{}, len(m.Items))
			for _, it := range m.Items {
				if _, ok := seen[it.ProdukID]; ok {
					continue
				}
				seen[it.ProdukID] = struct{}{}
				if err := s.mutasi.LockStokRow(ctx, tx, m.GudangAsalID, it.ProdukID); err != nil {
					return err
				}
			}
		}
		return s.mutasi.UpdateStatusInTx(ctx, tx, id, prev, domain.StatusDibatalkan, userID)
	})
	if err != nil {
		return err
	}
	s.logAudit(ctx, userID, "cancel", id, map[string]any{"status": string(prev)}, map[string]any{"status": "dibatalkan"})
	return nil
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

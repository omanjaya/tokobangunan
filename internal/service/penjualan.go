package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// PenjualanService - use case modul penjualan.
type PenjualanService struct {
	penjualan  *repo.PenjualanRepo
	produk     *repo.ProdukRepo
	mitra      *repo.MitraRepo
	gudang     *repo.GudangRepo
	satuan     *repo.SatuanRepo
	piutang    *repo.PiutangRepo
	appSetting *AppSettingService // optional; nil-safe
}

func NewPenjualanService(
	pj *repo.PenjualanRepo,
	pr *repo.ProdukRepo,
	mr *repo.MitraRepo,
	gr *repo.GudangRepo,
	sr *repo.SatuanRepo,
	piutangRepo *repo.PiutangRepo,
) *PenjualanService {
	return &PenjualanService{
		penjualan: pj, produk: pr, mitra: mr, gudang: gr, satuan: sr, piutang: piutangRepo,
	}
}

// SetAppSetting attach AppSettingService (untuk resolusi PajakConfig).
// Dipisah dari constructor supaya tidak break callers existing.
func (s *PenjualanService) SetAppSetting(as *AppSettingService) {
	s.appSetting = as
}

// Create - validate, resolve referensi, generate nomor, hitung total, persist.
func (s *PenjualanService) Create(
	ctx context.Context,
	userID int64,
	in dto.PenjualanCreateInput,
) (*domain.Penjualan, error) {
	if err := dto.Validate(in); err != nil {
		return nil, err
	}

	tgl, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}

	mitra, err := s.mitra.GetByID(ctx, in.MitraID)
	if err != nil {
		return nil, err
	}
	if !mitra.IsActive {
		return nil, domain.ErrMitraTidakDitemukan
	}

	gudang, err := s.gudang.GetByID(ctx, in.GudangID)
	if err != nil {
		return nil, err
	}
	if !gudang.IsActive {
		return nil, domain.ErrGudangNotFound
	}

	// Idempotency: kalau client_uuid sudah ada, return existing.
	clientUUID, err := parseOrNewUUID(in.ClientUUID)
	if err != nil {
		return nil, fmt.Errorf("client_uuid invalid: %w", err)
	}
	if existing, err := s.penjualan.GetByClientUUID(ctx, clientUUID); err == nil {
		_ = s.penjualan.LoadItems(ctx, existing)
		return existing, nil
	} else if !errors.Is(err, domain.ErrPenjualanNotFound) {
		return nil, err
	}

	// Resolve setiap item.
	items, err := s.resolveItems(ctx, in.Items)
	if err != nil {
		return nil, err
	}

	// Re-validate stok per produk (race condition guard sebelum insert).
	// Aggregasi qty_konversi per produk_id supaya jika multi-line untuk produk
	// yang sama, total kebutuhan dicek terhadap stok aktual.
	need := make(map[int64]float64, len(items))
	for _, it := range items {
		need[it.ProdukID] += it.QtyKonversi
	}
	for produkID, qtyNeed := range need {
		info, err := s.penjualan.StokInfoOf(ctx, gudang.ID, produkID)
		if err != nil {
			return nil, fmt.Errorf("cek stok produk %d: %w", produkID, err)
		}
		if info.Qty < qtyNeed {
			// Cari nama produk dari items yang sudah resolved untuk pesan jelas.
			nama := fmt.Sprintf("produk #%d", produkID)
			for _, it := range items {
				if it.ProdukID == produkID {
					nama = it.ProdukNama
					break
				}
			}
			return nil, fmt.Errorf("%w: %s (tersedia %.4f, butuh %.4f)",
				domain.ErrStokTidakCukup, nama, info.Qty, qtyNeed)
		}
	}

	// Build entity.
	p := &domain.Penjualan{
		Tanggal:     tgl,
		MitraID:     mitra.ID,
		GudangID:    gudang.ID,
		UserID:      userID,
		Items:       items,
		Diskon:      in.Diskon * 100, // Rupiah → cents
		StatusBayar: domain.StatusBayar(in.StatusBayar),
		Catatan:     strings.TrimSpace(in.Catatan),
		ClientUUID:  clientUUID,
	}
	if jt := strings.TrimSpace(in.JatuhTempo); jt != "" {
		t, err := time.Parse("2006-01-02", jt)
		if err != nil {
			return nil, fmt.Errorf("parse jatuh tempo: %w", err)
		}
		p.JatuhTempo = &t
	}
	// Resolve PPN dari app_setting kalau enabled di input dan global config aktif.
	if in.PPNEnabled && s.appSetting != nil {
		if cfg, err := s.appSetting.PajakConfig(ctx); err == nil && cfg != nil && cfg.PPNEnabled {
			p.PPNPersen = cfg.PPNPersen
		}
	}
	p.HitungTotal()

	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Cek limit kredit: outstanding mitra + total transaksi baru tidak boleh melebihi limit.
	if (p.StatusBayar == domain.StatusKredit || p.StatusBayar == domain.StatusSebagian) &&
		mitra.LimitKredit > 0 {
		outstanding, err := s.piutang.OutstandingByMitra(ctx, mitra.ID)
		if err != nil {
			return nil, fmt.Errorf("cek piutang outstanding: %w", err)
		}
		if outstanding+p.Total > mitra.LimitKredit {
			return nil, domain.ErrLimitKreditTerlampaui
		}
	}

	// Generate nomor.
	nomor, err := s.penjualan.NextNomor(ctx, gudang.Kode, tgl)
	if err != nil {
		return nil, err
	}
	p.NomorKwitansi = nomor

	if err := s.penjualan.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create penjualan: %w", err)
	}
	return p, nil
}

// Get penjualan by ID + load items.
func (s *PenjualanService) Get(ctx context.Context, id int64) (*domain.Penjualan, error) {
	p, err := s.penjualan.GetByID(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	if err := s.penjualan.LoadItems(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetByNomor + load items.
func (s *PenjualanService) GetByNomor(ctx context.Context, nomor string) (*domain.Penjualan, error) {
	p, err := s.penjualan.GetByNomor(ctx, nomor)
	if err != nil {
		return nil, err
	}
	if err := s.penjualan.LoadItems(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// List wrapper paginated.
func (s *PenjualanService) List(
	ctx context.Context, f repo.ListPenjualanFilter,
) (PageResult[domain.Penjualan], error) {
	items, total, err := s.penjualan.List(ctx, f)
	if err != nil {
		return PageResult[domain.Penjualan]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// ListWithRelations wrapper paginated yang sekaligus mem-bawa nama mitra +
// kode/nama gudang (1 query JOIN) — dipakai oleh list view supaya tidak N+1.
func (s *PenjualanService) ListWithRelations(
	ctx context.Context, f repo.ListPenjualanFilter,
) (PageResult[repo.PenjualanWithRelations], error) {
	items, total, err := s.penjualan.ListWithRelations(ctx, f)
	if err != nil {
		return PageResult[repo.PenjualanWithRelations]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Cancel - belum diimplementasi di Fase 2.
func (s *PenjualanService) Cancel(ctx context.Context, id int64) error {
	return errors.New("cancel penjualan belum diimplementasi pada Fase 2")
}

// StokInfoOf - posisi stok produk di gudang + stok_minimum.
func (s *PenjualanService) StokInfoOf(ctx context.Context, gudangID, produkID int64) (repo.StokInfo, error) {
	return s.penjualan.StokInfoOf(ctx, gudangID, produkID)
}

// OutstandingByMitra - total piutang outstanding mitra (cents). Wrapper service.
func (s *PenjualanService) OutstandingByMitra(ctx context.Context, mitraID int64) (int64, error) {
	return s.piutang.OutstandingByMitra(ctx, mitraID)
}

// PreviewNomor - hitung nomor kwitansi berikutnya untuk gudang+tanggal,
// tanpa melakukan side-effect. Berguna untuk display di form.
func (s *PenjualanService) PreviewNomor(ctx context.Context, gudangID int64, tanggal time.Time) (string, error) {
	g, err := s.gudang.GetByID(ctx, gudangID)
	if err != nil {
		return "", err
	}
	return s.penjualan.NextNomor(ctx, g.Kode, tanggal)
}

// ----- helpers ---------------------------------------------------------------

func (s *PenjualanService) resolveItems(
	ctx context.Context, in []dto.PenjualanItemInput,
) ([]domain.PenjualanItem, error) {
	out := make([]domain.PenjualanItem, 0, len(in))
	for i, raw := range in {
		if raw.Qty <= 0 {
			return nil, fmt.Errorf("item %d: %w", i+1, domain.ErrItemQtyInvalid)
		}
		produk, err := s.produk.GetByID(ctx, raw.ProdukID)
		if err != nil {
			return nil, fmt.Errorf("item %d produk: %w", i+1, err)
		}
		satuan, err := s.satuan.GetByID(ctx, raw.SatuanID)
		if err != nil {
			return nil, fmt.Errorf("item %d satuan: %w", i+1, err)
		}

		// Hitung qty_konversi: kalau satuan sama dengan satuan_kecil produk,
		// konversi = qty. Kalau satuan_besar, qty * faktor_konversi.
		qtyKonversi := raw.Qty
		if produk.SatuanBesarID != nil && *produk.SatuanBesarID == satuan.ID {
			qtyKonversi = raw.Qty * produk.FaktorKonversi
		}

		hargaCents := raw.HargaSatuan * 100 // Rupiah → cents
		diskonCents := raw.Diskon * 100     // Rupiah → cents
		gross := int64(raw.Qty*float64(hargaCents) + 0.5)
		if diskonCents < 0 {
			diskonCents = 0
		}
		if diskonCents > gross {
			diskonCents = gross
		}
		subtotal := gross - diskonCents

		out = append(out, domain.PenjualanItem{
			ProdukID:    produk.ID,
			ProdukNama:  produk.Nama,
			Qty:         raw.Qty,
			SatuanID:    satuan.ID,
			SatuanKode:  satuan.Kode,
			QtyKonversi: qtyKonversi,
			HargaSatuan: hargaCents,
			Diskon:      diskonCents,
			Subtotal:    subtotal,
		})
	}
	return out, nil
}

func parseOrNewUUID(s string) (uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return uuid.New(), nil
	}
	return uuid.Parse(s)
}

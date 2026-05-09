package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/format"
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
	audit      *AuditLogService   // optional; nil-safe (best-effort)
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

// SetAudit attach AuditLogService (best-effort, non-fatal post-commit).
func (s *PenjualanService) SetAudit(a *AuditLogService) { s.audit = a }

func (s *PenjualanService) logAudit(ctx context.Context, userID int64, aksi string, id int64, before, after any) {
	if s.audit == nil {
		return
	}
	uid := userID
	_ = s.audit.Record(ctx, RecordEntry{
		UserID: &uid, Aksi: aksi, Tabel: "penjualan", RecordID: id,
		Before: before, After: after,
	})
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

	// MitraID == 0 berarti POS walk-in: resolve ke mitra default "ECERAN".
	if in.MitraID == 0 {
		eceran, err := s.mitra.GetByKode(ctx, "ECERAN")
		if err != nil {
			return nil, fmt.Errorf("eceran mitra not found: %w", err)
		}
		in.MitraID = eceran.ID
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
			return nil, fmt.Errorf("%w: %s (tersedia %s, butuh %s)",
				domain.ErrStokTidakCukup, nama, format.Qty(info.Qty), format.Qty(qtyNeed))
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
	s.logAudit(ctx, userID, "create", p.ID, nil, map[string]any{
		"nomor_kwitansi": p.NomorKwitansi,
		"tanggal":        p.Tanggal,
		"mitra_id":       p.MitraID,
		"gudang_id":      p.GudangID,
		"total":          p.Total,
		"status_bayar":   string(p.StatusBayar),
		"items_count":    len(p.Items),
	})
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

// HasPembayaran wrapper repo — cek apakah invoice sudah ada pembayaran.
func (s *PenjualanService) HasPembayaran(ctx context.Context, id int64, tanggal time.Time) (bool, error) {
	return s.penjualan.HasPembayaran(ctx, id, tanggal)
}

// Update - revisi header + items penjualan existing.
// Guard: invoice tidak boleh sudah ada pembayaran atau sudah dibatalkan.
// Stok lama di-rollback, stok baru di-deduct dalam satu transaksi.
func (s *PenjualanService) Update(ctx context.Context, id int64, in dto.PenjualanCreateInput, userID int64) error {
	if err := dto.Validate(in); err != nil {
		return err
	}

	// Load existing untuk dapat tanggal (partition key) + before snapshot.
	existing, err := s.penjualan.GetByID(ctx, id, nil)
	if err != nil {
		return err
	}
	if existing.StatusBayar == domain.StatusBayarDibatalkan {
		return domain.ErrInvoiceDibatalkan
	}

	// Guard: tidak boleh edit kalau sudah ada pembayaran.
	hasPay, err := s.penjualan.HasPembayaran(ctx, id, existing.Tanggal)
	if err != nil {
		return err
	}
	if hasPay {
		return domain.ErrInvoiceLocked
	}

	// Resolve referensi (mitra/gudang/items).
	tgl, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return fmt.Errorf("parse tanggal: %w", err)
	}
	mitra, err := s.mitra.GetByID(ctx, in.MitraID)
	if err != nil {
		return err
	}
	if !mitra.IsActive {
		return domain.ErrMitraTidakDitemukan
	}
	gudang, err := s.gudang.GetByID(ctx, in.GudangID)
	if err != nil {
		return err
	}
	if !gudang.IsActive {
		return domain.ErrGudangNotFound
	}

	items, err := s.resolveItems(ctx, in.Items)
	if err != nil {
		return err
	}

	// Build entity (preserve immutable fields: ID, NomorKwitansi, Tanggal partition).
	p := &domain.Penjualan{
		ID:            existing.ID,
		NomorKwitansi: existing.NomorKwitansi,
		Tanggal:       existing.Tanggal, // partition key — tidak bisa diubah
		MitraID:       mitra.ID,
		GudangID:      gudang.ID,
		UserID:        existing.UserID,
		Items:         items,
		Diskon:        in.Diskon * 100,
		StatusBayar:   domain.StatusBayar(in.StatusBayar),
		Catatan:       strings.TrimSpace(in.Catatan),
		ClientUUID:    existing.ClientUUID,
	}
	_ = tgl // tanggal input diabaikan untuk partition safety
	if jt := strings.TrimSpace(in.JatuhTempo); jt != "" {
		t, err := time.Parse("2006-01-02", jt)
		if err != nil {
			return fmt.Errorf("parse jatuh tempo: %w", err)
		}
		p.JatuhTempo = &t
	}
	if in.PPNEnabled && s.appSetting != nil {
		if cfg, err := s.appSetting.PajakConfig(ctx); err == nil && cfg != nil && cfg.PPNEnabled {
			p.PPNPersen = cfg.PPNPersen
		}
	}
	p.HitungTotal()
	if err := p.Validate(); err != nil {
		return err
	}

	// Limit kredit guard.
	if (p.StatusBayar == domain.StatusKredit || p.StatusBayar == domain.StatusSebagian) &&
		mitra.LimitKredit > 0 {
		outstanding, err := s.piutang.OutstandingByMitra(ctx, mitra.ID)
		if err != nil {
			return fmt.Errorf("cek piutang outstanding: %w", err)
		}
		// Outstanding lama sudah include invoice ini (kalau status lama kredit/sebagian);
		// untuk simplifikasi: kalau status lama lunas, anggap outstanding murni;
		// kalau lama kredit/sebagian, kurangi total lama supaya tidak double count.
		effective := outstanding
		if existing.StatusBayar == domain.StatusKredit || existing.StatusBayar == domain.StatusSebagian {
			effective -= existing.Total
			if effective < 0 {
				effective = 0
			}
		}
		if effective+p.Total > mitra.LimitKredit {
			return domain.ErrLimitKreditTerlampaui
		}
	}

	err = pgx.BeginFunc(ctx, s.penjualan.Pool(), func(tx pgx.Tx) error {
		return s.penjualan.UpdateInTx(ctx, tx, p, items)
	})
	if err != nil {
		return fmt.Errorf("update penjualan: %w", err)
	}

	s.logAudit(ctx, userID, "update", p.ID,
		map[string]any{
			"mitra_id":     existing.MitraID,
			"total":        existing.Total,
			"status_bayar": string(existing.StatusBayar),
			"items_count":  len(existing.Items),
		},
		map[string]any{
			"mitra_id":     p.MitraID,
			"total":        p.Total,
			"status_bayar": string(p.StatusBayar),
			"items_count":  len(p.Items),
		})
	return nil
}

// Cancel - batalkan invoice. Guard: belum ada pembayaran & belum dibatalkan.
// Stok semua items di-rollback. canceled_at/by/reason diisi.
func (s *PenjualanService) Cancel(ctx context.Context, id int64, userID int64, alasan string) error {
	existing, err := s.penjualan.GetByID(ctx, id, nil)
	if err != nil {
		return err
	}
	if existing.StatusBayar == domain.StatusBayarDibatalkan {
		return domain.ErrInvoiceDibatalkan
	}
	hasPay, err := s.penjualan.HasPembayaran(ctx, id, existing.Tanggal)
	if err != nil {
		return err
	}
	if hasPay {
		return domain.ErrInvoiceLocked
	}

	err = pgx.BeginFunc(ctx, s.penjualan.Pool(), func(tx pgx.Tx) error {
		return s.penjualan.CancelInTx(ctx, tx, id, existing.Tanggal, userID, alasan)
	})
	if err != nil {
		return fmt.Errorf("cancel penjualan: %w", err)
	}

	s.logAudit(ctx, userID, "cancel", id,
		map[string]any{
			"status_bayar": string(existing.StatusBayar),
			"total":        existing.Total,
		},
		map[string]any{
			"status_bayar":  string(domain.StatusBayarDibatalkan),
			"cancel_reason": alasan,
		})
	return nil
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

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

// PembelianService orchestrasi use case pembelian + pembayaran supplier.
type PembelianService struct {
	pembelianRepo  *repo.PembelianRepo
	bayarRepo      *repo.PembayaranSupplierRepo
	supplierRepo   *repo.SupplierRepo
	produkRepo     *repo.ProdukRepo
	gudangRepo     *repo.GudangRepo
	satuanRepo     *repo.SatuanRepo
}

// NewPembelianService konstruktor.
func NewPembelianService(
	pembelianRepo *repo.PembelianRepo,
	bayarRepo *repo.PembayaranSupplierRepo,
	supplierRepo *repo.SupplierRepo,
	produkRepo *repo.ProdukRepo,
	gudangRepo *repo.GudangRepo,
	satuanRepo *repo.SatuanRepo,
) *PembelianService {
	return &PembelianService{
		pembelianRepo: pembelianRepo,
		bayarRepo:     bayarRepo,
		supplierRepo:  supplierRepo,
		produkRepo:    produkRepo,
		gudangRepo:    gudangRepo,
		satuanRepo:    satuanRepo,
	}
}

// Create generate nomor + insert. Stok diupdate via trigger DB.
// Input HargaSatuan & Diskon adalah Rupiah utuh (akan dikonversi ke cents).
func (s *PembelianService) Create(ctx context.Context, in dto.PembelianCreateInput, userID int64) (*domain.Pembelian, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}

	// Validasi supplier & gudang exist.
	sup, err := s.supplierRepo.GetByID(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if !sup.IsActive {
		return nil, fmt.Errorf("supplier nonaktif")
	}
	if _, err := s.gudangRepo.GetByID(ctx, in.GudangID); err != nil {
		return nil, err
	}

	// Build items (snapshot nama produk + kode satuan, hitung subtotal).
	items := make([]domain.PembelianItem, 0, len(in.Items))
	var subtotalCents int64
	for _, ii := range in.Items {
		prod, err := s.produkRepo.GetByID(ctx, ii.ProdukID)
		if err != nil {
			return nil, err
		}
		sat, err := s.satuanRepo.GetByID(ctx, ii.SatuanID)
		if err != nil {
			return nil, err
		}
		// Faktor konversi: bila satuan = satuan_besar produk → kalikan,
		// kalau bukan, asume 1 (qty_konversi = qty).
		konversi := 1.0
		if prod.SatuanBesarID != nil && *prod.SatuanBesarID == ii.SatuanID {
			konversi = prod.FaktorKonversi
		}
		qtyKonversi := ii.Qty * konversi

		hargaCents := ii.HargaSatuan * 100
		subCents := int64(ii.Qty * float64(hargaCents))
		subtotalCents += subCents

		items = append(items, domain.PembelianItem{
			ProdukID:    prod.ID,
			ProdukNama:  prod.Nama,
			Qty:         ii.Qty,
			SatuanID:    sat.ID,
			SatuanKode:  sat.Kode,
			QtyKonversi: qtyKonversi,
			HargaSatuan: hargaCents,
			Subtotal:    subCents,
		})
	}

	diskonCents := in.Diskon * 100
	totalCents := subtotalCents - diskonCents

	var jatuhTempo *time.Time
	if v := strings.TrimSpace(in.JatuhTempo); v != "" {
		jt, err := time.Parse("2006-01-02", v)
		if err == nil {
			jatuhTempo = &jt
		}
	}

	// Generate nomor.
	nomor, err := s.pembelianRepo.NextNomor(ctx, tanggal)
	if err != nil {
		return nil, err
	}

	p := &domain.Pembelian{
		NomorPembelian: nomor,
		Tanggal:        tanggal,
		SupplierID:     in.SupplierID,
		GudangID:       in.GudangID,
		UserID:         userID,
		Items:          items,
		Subtotal:       subtotalCents,
		Diskon:         diskonCents,
		DPP:            totalCents,
		PPNPersen:      0,
		PPNAmount:      0,
		Total:          totalCents,
		StatusBayar:    domain.StatusBayarPembelian(in.StatusBayar),
		JatuhTempo:     jatuhTempo,
		Catatan:        strings.TrimSpace(in.Catatan),
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.pembelianRepo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get load pembelian + items.
func (s *PembelianService) Get(ctx context.Context, id int64) (*domain.Pembelian, error) {
	p, err := s.pembelianRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.pembelianRepo.LoadItems(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// List paginated.
func (s *PembelianService) List(ctx context.Context, f repo.ListPembelianFilter) (PageResult[domain.Pembelian], error) {
	items, total, err := s.pembelianRepo.List(ctx, f)
	if err != nil {
		return PageResult[domain.Pembelian]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Cancel belum didukung di Fase 5.
func (s *PembelianService) Cancel(ctx context.Context, id int64) error {
	return domain.ErrPembelianCancelBelum
}

// RecordPayment insert pembayaran + recompute status pembelian.
func (s *PembelianService) RecordPayment(ctx context.Context, in dto.PembayaranSupplierInput, userID int64) (*domain.PembayaranSupplier, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	jumlahCents := in.Jumlah * 100

	p := &domain.PembayaranSupplier{
		PembelianID: in.PembelianID,
		SupplierID:  in.SupplierID,
		Tanggal:     tanggal,
		Jumlah:      jumlahCents,
		Metode:      strings.ToLower(strings.TrimSpace(in.Metode)),
		Referensi:   strings.TrimSpace(in.Referensi),
		UserID:      userID,
		Catatan:     strings.TrimSpace(in.Catatan),
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.bayarRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	// Recompute status untuk pembelian terkait.
	if p.PembelianID != nil {
		if err := s.recomputeStatusBayar(ctx, *p.PembelianID); err != nil {
			return p, err
		}
	}
	return p, nil
}

// HistoryPembayaran list pembayaran utk satu pembelian.
func (s *PembelianService) HistoryPembayaran(ctx context.Context, pembelianID int64) ([]domain.PembayaranSupplier, error) {
	return s.bayarRepo.ListByPembelian(ctx, pembelianID)
}

// SisaPembelian total - sum bayar.
func (s *PembelianService) SisaPembelian(ctx context.Context, p *domain.Pembelian) (int64, error) {
	sum, err := s.bayarRepo.SumByPembelian(ctx, p.ID)
	if err != nil {
		return 0, err
	}
	sisa := p.Total - sum
	if sisa < 0 {
		sisa = 0
	}
	return sisa, nil
}

func (s *PembelianService) recomputeStatusBayar(ctx context.Context, pembelianID int64) error {
	p, err := s.pembelianRepo.GetByID(ctx, pembelianID)
	if err != nil {
		if errors.Is(err, domain.ErrPembelianTidakDitemukan) {
			return nil
		}
		return err
	}
	sum, err := s.bayarRepo.SumByPembelian(ctx, pembelianID)
	if err != nil {
		return err
	}
	var newStatus domain.StatusBayarPembelian
	switch {
	case sum >= p.Total:
		newStatus = domain.StatusBeliLunas
	case sum > 0:
		newStatus = domain.StatusBeliSebagian
	default:
		newStatus = domain.StatusBeliKredit
	}
	if newStatus == p.StatusBayar {
		return nil
	}
	return s.pembelianRepo.UpdateStatusBayar(ctx, pembelianID, newStatus)
}

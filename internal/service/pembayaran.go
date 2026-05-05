package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// PembayaranService orchestrasi pencatatan pembayaran customer (mitra).
type PembayaranService struct {
	pembayaranRepo *repo.PembayaranRepo
	penjualanRepo  *repo.PenjualanRepo
	mitraRepo      *repo.MitraRepo
	piutangRepo    *repo.PiutangRepo
}

// NewPembayaranService konstruktor.
func NewPembayaranService(
	pembayaranRepo *repo.PembayaranRepo,
	penjualanRepo *repo.PenjualanRepo,
	mitraRepo *repo.MitraRepo,
	piutangRepo *repo.PiutangRepo,
) *PembayaranService {
	return &PembayaranService{
		pembayaranRepo: pembayaranRepo,
		penjualanRepo:  penjualanRepo,
		mitraRepo:      mitraRepo,
		piutangRepo:    piutangRepo,
	}
}

// Record catat satu pembayaran (penjualan_id optional).
// Input.Jumlah dalam Rupiah utuh; service konversi ke cents.
func (s *PembayaranService) Record(ctx context.Context, in dto.PembayaranCreateInput, userID int64) (*domain.Pembayaran, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.mitraRepo.GetByID(ctx, in.MitraID); err != nil {
		return nil, err
	}
	jumlahCents := in.Jumlah * 100

	clientUUID, err := parseOrNewPembayaranUUID(in.ClientUUID)
	if err != nil {
		return nil, err
	}
	// Idempotency.
	if existing, err := s.pembayaranRepo.GetByClientUUID(ctx, clientUUID); err == nil && existing != nil {
		return existing, nil
	}

	p := &domain.Pembayaran{
		MitraID:    in.MitraID,
		Tanggal:    tanggal,
		Jumlah:     jumlahCents,
		Metode:     domain.MetodeBayar(strings.ToLower(strings.TrimSpace(in.Metode))),
		Referensi:  strings.TrimSpace(in.Referensi),
		UserID:     userID,
		Catatan:    strings.TrimSpace(in.Catatan),
		ClientUUID: clientUUID,
	}

	// Bila penjualan_id ada → load + validasi.
	if in.PenjualanID != nil && *in.PenjualanID > 0 {
		pj, err := s.penjualanRepo.GetByID(ctx, *in.PenjualanID, nil)
		if err != nil {
			return nil, err
		}
		if pj.MitraID != in.MitraID {
			return nil, fmt.Errorf("penjualan tidak milik mitra ini")
		}
		// Cek outstanding cukup.
		dibayar, err := s.pembayaranRepo.SumByPenjualan(ctx, pj.ID, pj.Tanggal)
		if err != nil {
			return nil, err
		}
		outstanding := pj.Total - dibayar
		if jumlahCents > outstanding {
			return nil, domain.ErrJumlahLebihDariOutstanding
		}
		idCopy := pj.ID
		tgCopy := pj.Tanggal
		p.PenjualanID = &idCopy
		p.PenjualanTanggal = &tgCopy
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.pembayaranRepo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// RecordBatch alokasi FIFO ke invoice tertua belum lunas dari mitra.
// Sisa allocation kalau ada → return error ErrJumlahLebihDariOutstanding.
func (s *PembayaranService) RecordBatch(ctx context.Context, in dto.PembayaranBatchInput, userID int64) ([]domain.Pembayaran, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.mitraRepo.GetByID(ctx, in.MitraID); err != nil {
		return nil, err
	}
	sisa := in.Jumlah * 100
	metode := domain.MetodeBayar(strings.ToLower(strings.TrimSpace(in.Metode)))
	if !domain.IsValidMetodeBayar(string(metode)) {
		return nil, domain.ErrPembayaranInvalid
	}
	if sisa <= 0 {
		return nil, domain.ErrPembayaranInvalid
	}

	// Ambil invoice belum lunas mitra (FIFO oldest first).
	var invs []domain.PiutangInvoice
	if s.piutangRepo != nil {
		invs, _, err = s.piutangRepo.InvoicesByMitra(ctx, in.MitraID, repo.ListInvoiceFilter{Page: 1, PerPage: 100})
		if err != nil {
			return nil, err
		}
	} else {
		invs, err = s.fetchOpenInvoices(ctx, in.MitraID)
		if err != nil {
			return nil, err
		}
	}

	out := make([]domain.Pembayaran, 0, len(invs))
	ref := strings.TrimSpace(in.Referensi)
	catatan := strings.TrimSpace(in.Catatan)

	for _, inv := range invs {
		if sisa <= 0 {
			break
		}
		alloc := inv.Outstanding
		if alloc > sisa {
			alloc = sisa
		}
		clientUUID := uuid.New()
		idCopy := inv.PenjualanID
		tgCopy := inv.PenjualanTanggal
		p := domain.Pembayaran{
			PenjualanID:      &idCopy,
			PenjualanTanggal: &tgCopy,
			MitraID:          in.MitraID,
			Tanggal:          tanggal,
			Jumlah:           alloc,
			Metode:           metode,
			Referensi:        ref,
			UserID:           userID,
			Catatan:          catatan,
			ClientUUID:       clientUUID,
		}
		if err := p.Validate(); err != nil {
			return out, err
		}
		if err := s.pembayaranRepo.Create(ctx, &p); err != nil {
			return out, err
		}
		out = append(out, p)
		sisa -= alloc
	}
	if sisa > 0 {
		return out, domain.ErrJumlahLebihDariOutstanding
	}
	return out, nil
}

// fetchOpenInvoices fallback: query penjualan kredit/sebagian + dibayar per invoice.
func (s *PembayaranService) fetchOpenInvoices(ctx context.Context, mitraID int64) ([]domain.PiutangInvoice, error) {
	// Pakai PenjualanRepo.List dengan filter mitra+status, lalu hitung dibayar.
	mitraIDCopy := mitraID
	out := []domain.PiutangInvoice{}
	for _, status := range []string{"kredit", "sebagian"} {
		statusCopy := status
		f := repo.ListPenjualanFilter{
			MitraID: &mitraIDCopy,
			Status:  &statusCopy,
			Page:    1,
			PerPage: 100,
		}
		items, _, err := s.penjualanRepo.List(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, pj := range items {
			dibayar, err := s.pembayaranRepo.SumByPenjualan(ctx, pj.ID, pj.Tanggal)
			if err != nil {
				return nil, err
			}
			outs := pj.Total - dibayar
			if outs <= 0 {
				continue
			}
			out = append(out, domain.PiutangInvoice{
				PenjualanID:      pj.ID,
				PenjualanTanggal: pj.Tanggal,
				NomorKwitansi:    pj.NomorKwitansi,
				Tanggal:          pj.Tanggal,
				JatuhTempo:       pj.JatuhTempo,
				Total:            pj.Total,
				Dibayar:          dibayar,
				Outstanding:      outs,
			})
		}
	}
	// Sort FIFO by tanggal asc, id asc.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if a.Tanggal.After(b.Tanggal) || (a.Tanggal.Equal(b.Tanggal) && a.PenjualanID > b.PenjualanID) {
				out[j-1], out[j] = b, a
			} else {
				break
			}
		}
	}
	return out, nil
}

// Get satu pembayaran.
func (s *PembayaranService) Get(ctx context.Context, id int64) (*domain.Pembayaran, error) {
	return s.pembayaranRepo.GetByID(ctx, id)
}

// ListByMitra paginated.
func (s *PembayaranService) ListByMitra(ctx context.Context, mitraID int64, f repo.ListPembayaranFilter) (PageResult[domain.Pembayaran], error) {
	items, total, err := s.pembayaranRepo.ListByMitra(ctx, mitraID, f)
	if err != nil {
		return PageResult[domain.Pembayaran]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Cancel belum didukung di Fase 4.
func (s *PembayaranService) Cancel(ctx context.Context, id int64) error {
	return domain.ErrPembayaranCancelBelum
}

func parseOrNewPembayaranUUID(s string) (uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return uuid.New(), nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		// Fallback ke generate baru kalau format invalid.
		return uuid.New(), nil
	}
	return u, nil
}


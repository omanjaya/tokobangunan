package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// PembayaranService orchestrasi pencatatan pembayaran customer (mitra).
type PembayaranService struct {
	pool           *pgxpool.Pool
	pembayaranRepo *repo.PembayaranRepo
	penjualanRepo  *repo.PenjualanRepo
	mitraRepo      *repo.MitraRepo
	piutangRepo    *repo.PiutangRepo
	audit          *AuditLogService // nullable
}

// NewPembayaranService konstruktor.
func NewPembayaranService(
	pool *pgxpool.Pool,
	pembayaranRepo *repo.PembayaranRepo,
	penjualanRepo *repo.PenjualanRepo,
	mitraRepo *repo.MitraRepo,
	piutangRepo *repo.PiutangRepo,
) *PembayaranService {
	return &PembayaranService{
		pool:           pool,
		pembayaranRepo: pembayaranRepo,
		penjualanRepo:  penjualanRepo,
		mitraRepo:      mitraRepo,
		piutangRepo:    piutangRepo,
	}
}

// SetAudit attach AuditLogService (best-effort logging post-commit).
func (s *PembayaranService) SetAudit(a *AuditLogService) { s.audit = a }

// Record catat satu pembayaran (penjualan_id optional).
// Input.Jumlah dalam Rupiah utuh; service konversi ke cents.
//
// Concurrency: bila penjualan_id ada, validate+insert dijalankan di dalam satu
// transaksi dengan SELECT ... FOR UPDATE pada baris penjualan terkait. Ini
// mencegah race-condition overpayment kalau dua request paralel melihat
// outstanding lama yang sama.
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
	// Convert breakdown rupiah → cents kalau ada.
	if len(in.MetodeBreakdown) > 0 {
		bd := make([]domain.MetodePembayaranBreakdown, len(in.MetodeBreakdown))
		var sumCents int64
		for i, b := range in.MetodeBreakdown {
			cents := b.Jumlah * 100
			bd[i] = domain.MetodePembayaranBreakdown{
				Metode:    strings.ToLower(strings.TrimSpace(b.Metode)),
				Jumlah:    cents,
				Referensi: strings.TrimSpace(b.Referensi),
			}
			sumCents += cents
		}
		if sumCents != jumlahCents {
			return nil, fmt.Errorf("%w: total breakdown tidak sama dengan jumlah header", domain.ErrPembayaranInvalid)
		}
		p.MetodeBreakdown = bd
	}

	// Tanpa penjualan_id → no-locking path (tabungan/setoran umum).
	if !(in.PenjualanID != nil && *in.PenjualanID > 0) {
		if err := p.Validate(); err != nil {
			return nil, err
		}
		if err := s.pembayaranRepo.Create(ctx, p); err != nil {
			return nil, err
		}
		s.bestEffortAudit(ctx, userID, "create", p, nil)
		return p, nil
	}

	// Resolve invoice tanggal di luar tx (read-only).
	pj, err := s.penjualanRepo.GetByID(ctx, *in.PenjualanID, nil)
	if err != nil {
		return nil, err
	}
	if pj.MitraID != in.MitraID {
		return nil, fmt.Errorf("penjualan tidak milik mitra ini")
	}

	// Tx + FOR UPDATE: lock invoice supaya pengecekan sum + insert atomik.
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var total int64
		// Composite key (id, tanggal) supaya partition pruning kena.
		if err := tx.QueryRow(ctx,
			`SELECT total FROM penjualan WHERE id = $1 AND tanggal = $2 FOR UPDATE`,
			pj.ID, pj.Tanggal,
		).Scan(&total); err != nil {
			return fmt.Errorf("lock penjualan: %w", err)
		}
		var dibayar int64
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran
			 WHERE penjualan_id = $1 AND penjualan_tanggal = $2`,
			pj.ID, pj.Tanggal,
		).Scan(&dibayar); err != nil {
			return fmt.Errorf("sum existing pembayaran: %w", err)
		}
		outstanding := total - dibayar
		if jumlahCents > outstanding {
			return domain.ErrJumlahLebihDariOutstanding
		}
		idCopy := pj.ID
		tgCopy := pj.Tanggal
		p.PenjualanID = &idCopy
		p.PenjualanTanggal = &tgCopy
		if err := p.Validate(); err != nil {
			return err
		}
		// Trigger DB recompute status_bayar otomatis (lihat migration 0017).
		return s.pembayaranRepo.CreateInTx(ctx, tx, p)
	})
	if err != nil {
		return nil, err
	}
	s.bestEffortAudit(ctx, userID, "create", p, nil)
	return p, nil
}

// RecordBatch alokasi FIFO ke invoice tertua belum lunas dari mitra.
// Sisa allocation kalau ada → return error ErrJumlahLebihDariOutstanding.
//
// Seluruh batch dijalankan di dalam satu tx; tiap invoice di-lock FOR UPDATE
// sebelum allocate supaya overpay race ditolak deterministik.
func (s *PembayaranService) RecordBatch(ctx context.Context, in dto.PembayaranBatchInput, userID int64) ([]domain.Pembayaran, error) {
	tanggal, err := time.Parse("2006-01-02", in.Tanggal)
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}
	if _, err := s.mitraRepo.GetByID(ctx, in.MitraID); err != nil {
		return nil, err
	}
	totalAlloc := in.Jumlah * 100
	metode := domain.MetodeBayar(strings.ToLower(strings.TrimSpace(in.Metode)))
	if !domain.IsValidMetodeBayar(string(metode)) {
		return nil, domain.ErrPembayaranInvalid
	}
	if totalAlloc <= 0 {
		return nil, domain.ErrPembayaranInvalid
	}

	// Ambil invoice belum lunas mitra (FIFO oldest first) — di luar tx
	// (read-only, akan re-validate per invoice di dalam tx).
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

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		sisa := totalAlloc
		for _, inv := range invs {
			if sisa <= 0 {
				break
			}
			// Lock invoice + recompute outstanding di dalam tx (state authoritative).
			var total int64
			if err := tx.QueryRow(ctx,
				`SELECT total FROM penjualan WHERE id = $1 AND tanggal = $2 FOR UPDATE`,
				inv.PenjualanID, inv.PenjualanTanggal,
			).Scan(&total); err != nil {
				return fmt.Errorf("lock penjualan %d: %w", inv.PenjualanID, err)
			}
			var dibayar int64
			if err := tx.QueryRow(ctx,
				`SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran
				 WHERE penjualan_id = $1 AND penjualan_tanggal = $2`,
				inv.PenjualanID, inv.PenjualanTanggal,
			).Scan(&dibayar); err != nil {
				return fmt.Errorf("sum pembayaran %d: %w", inv.PenjualanID, err)
			}
			outs := total - dibayar
			if outs <= 0 {
				continue
			}
			alloc := outs
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
				return err
			}
			if err := s.pembayaranRepo.CreateInTx(ctx, tx, &p); err != nil {
				return err
			}
			out = append(out, p)
			sisa -= alloc
		}
		if sisa > 0 {
			return domain.ErrJumlahLebihDariOutstanding
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	for i := range out {
		s.bestEffortAudit(ctx, userID, "create", &out[i], nil)
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

func (s *PembayaranService) bestEffortAudit(ctx context.Context, userID int64, aksi string, p *domain.Pembayaran, before any) {
	if s.audit == nil {
		return
	}
	uid := userID
	after := map[string]any{
		"id":                p.ID,
		"penjualan_id":      p.PenjualanID,
		"penjualan_tanggal": p.PenjualanTanggal,
		"mitra_id":          p.MitraID,
		"tanggal":           p.Tanggal,
		"jumlah":            p.Jumlah,
		"metode":            string(p.Metode),
		"referensi":         p.Referensi,
		"catatan":           p.Catatan,
	}
	_ = s.audit.Record(ctx, RecordEntry{
		UserID:   &uid,
		Aksi:     aksi,
		Tabel:    "pembayaran",
		RecordID: p.ID,
		Before:   before,
		After:    after,
	})
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

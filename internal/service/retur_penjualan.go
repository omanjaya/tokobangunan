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
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// ReturPenjualanService - use case retur penjualan customer.
type ReturPenjualanService struct {
	retur     *repo.ReturPenjualanRepo
	penjualan *repo.PenjualanRepo
	produk    *repo.ProdukRepo
	satuan    *repo.SatuanRepo
	gudang    *repo.GudangRepo
	audit     *AuditLogService // optional
}

// NewReturPenjualanService konstruktor.
func NewReturPenjualanService(
	rr *repo.ReturPenjualanRepo,
	pj *repo.PenjualanRepo,
	pr *repo.ProdukRepo,
	sr *repo.SatuanRepo,
	gr *repo.GudangRepo,
) *ReturPenjualanService {
	return &ReturPenjualanService{retur: rr, penjualan: pj, produk: pr, satuan: sr, gudang: gr}
}

func (s *ReturPenjualanService) SetAudit(a *AuditLogService) { s.audit = a }

func (s *ReturPenjualanService) logAudit(ctx context.Context, userID int64, aksi string, id int64, after any) {
	if s.audit == nil {
		return
	}
	uid := userID
	_ = s.audit.Record(ctx, RecordEntry{
		UserID: &uid, Aksi: aksi, Tabel: "retur_penjualan", RecordID: id, After: after,
	})
}

// Get retur + load items.
func (s *ReturPenjualanService) Get(ctx context.Context, id int64) (*domain.ReturPenjualan, error) {
	r, err := s.retur.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.retur.LoadItems(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// List retur paginated.
func (s *ReturPenjualanService) List(
	ctx context.Context, f repo.ListReturPenjualanFilter,
) (PageResult[repo.ReturWithRelations], error) {
	items, total, err := s.retur.ListWithRelations(ctx, f)
	if err != nil {
		return PageResult[repo.ReturWithRelations]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

// Create - bikin retur baru. Stok bertambah, pembayaran refund (negatif) tercatat.
func (s *ReturPenjualanService) Create(
	ctx context.Context, in dto.ReturPenjualanInput, userID int64,
) (*domain.ReturPenjualan, error) {
	if errs := in.Validate(); errs != nil {
		return nil, fmt.Errorf("validasi: %v", errs)
	}
	tgl, err := time.Parse("2006-01-02", strings.TrimSpace(in.Tanggal))
	if err != nil {
		return nil, fmt.Errorf("parse tanggal: %w", err)
	}

	// Load invoice asal (untuk dapat tanggal partition + gudang + mitra).
	pj, err := s.penjualan.GetByID(ctx, in.PenjualanID, nil)
	if err != nil {
		return nil, err
	}
	if pj.StatusBayar == domain.StatusBayarDibatalkan {
		return nil, domain.ErrReturInvoiceDibatalkan
	}
	if err := s.penjualan.LoadItems(ctx, pj); err != nil {
		return nil, err
	}
	itemMap := make(map[int64]*domain.PenjualanItem, len(pj.Items))
	for i := range pj.Items {
		itemMap[pj.Items[i].ID] = &pj.Items[i]
	}

	gudang, err := s.gudang.GetByID(ctx, pj.GudangID)
	if err != nil {
		return nil, err
	}

	// Filter input items dengan qty > 0 + resolve harga & qty_konversi.
	type prepItem struct {
		raw       dto.ReturItemInput
		invoice   *domain.PenjualanItem
		qtyKonv   float64
		hargaSat  int64
		subtotal  int64
		satuanKey int64
	}
	var prep []prepItem
	for _, it := range in.Items {
		if it.Qty <= 0 {
			continue
		}
		invIt, ok := itemMap[it.PenjualanItemID]
		if !ok {
			return nil, domain.ErrReturItemNotFound
		}
		// Resolve qty_konversi: kalau satuan berbeda dengan invoice line, konversi via produk.
		qtyKonv := it.Qty
		if it.SatuanID != invIt.SatuanID {
			produk, err := s.produk.GetByID(ctx, invIt.ProdukID)
			if err != nil {
				return nil, err
			}
			if produk.SatuanBesarID != nil && *produk.SatuanBesarID == it.SatuanID {
				qtyKonv = it.Qty * produk.FaktorKonversi
			}
		} else {
			// pakai ratio invoice qty_konversi/qty kalau qty>0
			if invIt.Qty > 0 {
				qtyKonv = it.Qty * (invIt.QtyKonversi / invIt.Qty)
			}
		}
		// Harga satuan: pakai harga di invoice (per unit yg dipilih). Asumsi
		// satuan retur sama dengan invoice line; kalau beda, fallback proporsional.
		hargaSat := invIt.HargaSatuan
		gross := int64(it.Qty*float64(hargaSat) + 0.5)
		prep = append(prep, prepItem{
			raw:       it,
			invoice:   invIt,
			qtyKonv:   qtyKonv,
			hargaSat:  hargaSat,
			subtotal:  gross,
			satuanKey: it.SatuanID,
		})
	}
	if len(prep) == 0 {
		return nil, domain.ErrReturPenjualanKosong
	}

	// Generate nomor retur (best-effort; bisa konflik race, tapi UNIQUE di DB).
	nomor, err := s.retur.NextNomor(ctx, gudang.Kode, tgl)
	if err != nil {
		return nil, err
	}

	var mitraIDPtr *int64
	if pj.MitraID > 0 {
		m := pj.MitraID
		mitraIDPtr = &m
	}

	r := &domain.ReturPenjualan{
		NomorRetur:       nomor,
		PenjualanID:      pj.ID,
		PenjualanTanggal: pj.Tanggal,
		MitraID:          mitraIDPtr,
		GudangID:         pj.GudangID,
		Tanggal:          tgl,
		Alasan:           strings.TrimSpace(in.Alasan),
		Catatan:          strings.TrimSpace(in.Catatan),
		UserID:           userID,
	}

	err = pgx.BeginFunc(ctx, s.retur.Pool(), func(tx pgx.Tx) error {
		// Lock invoice supaya cek qty available stabil.
		var statusBayar string
		if e := tx.QueryRow(ctx,
			`SELECT status_bayar FROM penjualan WHERE id=$1 AND tanggal=$2 FOR UPDATE`,
			pj.ID, pj.Tanggal,
		).Scan(&statusBayar); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return domain.ErrPenjualanNotFound
			}
			return fmt.Errorf("lock penjualan: %w", e)
		}
		if statusBayar == string(domain.StatusBayarDibatalkan) {
			return domain.ErrReturInvoiceDibatalkan
		}

		// Build items + cek qty available per penjualan_item.
		var subtotalRefund int64
		items := make([]domain.ReturPenjualanItem, 0, len(prep))
		for _, p := range prep {
			already, e := s.retur.SumQtyByPenjualanItemTx(ctx, tx, p.invoice.ID)
			if e != nil {
				return e
			}
			available := p.invoice.QtyKonversi - already
			if p.qtyKonv > available+1e-6 {
				return fmt.Errorf("%w: %s (tersedia %.4f, butuh %.4f)",
					domain.ErrReturQtyMelebihi, p.invoice.ProdukNama, available, p.qtyKonv)
			}
			items = append(items, domain.ReturPenjualanItem{
				PenjualanItemID: p.invoice.ID,
				ProdukID:        p.invoice.ProdukID,
				Qty:             p.raw.Qty,
				QtyKonversi:     p.qtyKonv,
				SatuanID:        p.satuanKey,
				HargaSatuan:     p.hargaSat,
				Subtotal:        p.subtotal,
			})
			subtotalRefund += p.subtotal
		}
		r.SubtotalRefund = subtotalRefund
		r.Items = items

		// Insert retur header + items.
		if e := s.retur.CreateInTx(ctx, tx, r); e != nil {
			return e
		}

		// Tambah stok per produk (UPSERT seperti rollback).
		usage := make(map[int64]float64, len(items))
		for _, it := range items {
			usage[it.ProdukID] += it.QtyKonversi
		}
		for pid, qty := range usage {
			if _, e := tx.Exec(ctx,
				`INSERT INTO stok (gudang_id, produk_id, qty, updated_at)
				 VALUES ($1,$2,$3,now())
				 ON CONFLICT (gudang_id, produk_id)
				 DO UPDATE SET qty = stok.qty + EXCLUDED.qty, updated_at = now()`,
				pj.GudangID, pid, qty,
			); e != nil {
				return fmt.Errorf("update stok produk %d: %w", pid, e)
			}
		}

		// Insert pembayaran refund (negatif). Direct INSERT (bypass Validate
		// domain karena metode='retur' dan jumlah negatif). Trigger DB akan
		// recompute status_bayar penjualan.
		if pj.MitraID > 0 && subtotalRefund > 0 {
			refundUUID, _ := uuid.NewRandom()
			if _, e := tx.Exec(ctx,
				`INSERT INTO pembayaran
					(penjualan_id, penjualan_tanggal, mitra_id, tanggal, jumlah, metode,
					 referensi, user_id, catatan, client_uuid)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				pj.ID, pj.Tanggal, pj.MitraID, tgl, -subtotalRefund, "retur",
				r.NomorRetur, userID, "Refund retur "+r.NomorRetur, refundUUID,
			); e != nil {
				return fmt.Errorf("insert pembayaran refund: %w", e)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create retur: %w", err)
	}

	s.logAudit(ctx, userID, "create", r.ID, map[string]any{
		"nomor_retur":     r.NomorRetur,
		"penjualan_id":    r.PenjualanID,
		"subtotal_refund": r.SubtotalRefund,
		"items_count":     len(r.Items),
	})
	return r, nil
}

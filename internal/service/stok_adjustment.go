package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// StokAdjustmentService - use case penyesuaian stok single-step.
// Setiap call Create:
//  1. Validate input.
//  2. Resolve qty_konversi (satuan_kecil → 1:1, satuan_besar → x faktor).
//  3. Lock + UPSERT row stok (qty boleh negatif final, sesuai kebijakan).
//  4. Insert baris stok_adjustment (audit trail).
//  5. (Opsional) tulis ke audit_log via AuditLogService.
type StokAdjustmentService struct {
	pool   *pgxpool.Pool
	repo   *repo.AdjRepo
	produk *repo.ProdukRepo
	satuan *repo.SatuanRepo
	audit  *AuditLogService // nullable
}

// NewStokAdjustmentService konstruktor. audit boleh nil bila tidak ingin
// double-log (table stok_adjustment sudah jadi audit utama untuk modul ini).
func NewStokAdjustmentService(
	pool *pgxpool.Pool,
	r *repo.AdjRepo,
	produk *repo.ProdukRepo,
	satuan *repo.SatuanRepo,
	audit *AuditLogService,
) *StokAdjustmentService {
	return &StokAdjustmentService{
		pool:   pool,
		repo:   r,
		produk: produk,
		satuan: satuan,
		audit:  audit,
	}
}

// Create eksekusi penyesuaian stok dalam 1 transaction.
func (s *StokAdjustmentService) Create(
	ctx context.Context, userID int64, in dto.StokAdjustmentInput,
) (*domain.StokAdjustment, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, errors.New("user id wajib")
	}

	// Resolve produk + faktor konversi.
	produk, err := s.produk.GetByID(ctx, in.ProdukID)
	if err != nil {
		return nil, fmt.Errorf("produk: %w", err)
	}
	satuan, err := s.satuan.GetByID(ctx, in.SatuanID)
	if err != nil {
		return nil, fmt.Errorf("satuan: %w", err)
	}

	// Tentukan qty_konversi (selalu mengacu ke satuan_kecil produk).
	qtyKonversi := in.Qty
	switch {
	case satuan.ID == produk.SatuanKecilID:
		qtyKonversi = in.Qty
	case produk.SatuanBesarID != nil && satuan.ID == *produk.SatuanBesarID:
		qtyKonversi = in.Qty * produk.FaktorKonversi
	default:
		return nil, domain.ErrAdjSatuanTidakCocok
	}

	a := &domain.StokAdjustment{
		GudangID:    in.GudangID,
		ProdukID:    in.ProdukID,
		SatuanID:    in.SatuanID,
		Qty:         in.Qty,
		QtyKonversi: qtyKonversi,
		Kategori:    in.Kategori,
		Alasan:      domain.AdjAlasanDefault(in.Kategori),
		UserID:      userID,
	}
	if c := strings.TrimSpace(in.Catatan); c != "" {
		a.Catatan = &c
	}

	// 1 tx: lock stok, apply delta, insert audit row.
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Lock row stok bila ada (boleh belum ada → perlakuan delta langsung).
		var current float64
		err := tx.QueryRow(ctx,
			`SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2 FOR UPDATE`,
			in.GudangID, in.ProdukID,
		).Scan(&current)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Belum ada row → akan di-insert via UPSERT di bawah.
			// Untuk delta negatif tanpa stok awal: tolak.
			if qtyKonversi < 0 {
				return fmt.Errorf("%w: produk %d (tersedia 0)",
					domain.ErrAdjStokTidakCukup, in.ProdukID)
			}
		case err != nil:
			return fmt.Errorf("lock stok: %w", err)
		default:
			if qtyKonversi < 0 && current+qtyKonversi < 0 {
				return fmt.Errorf("%w: produk %d (tersedia %s, butuh %s)",
					domain.ErrAdjStokTidakCukup, in.ProdukID, format.Qty(current), format.Qty(-qtyKonversi))
			}
		}

		// UPSERT stok dgn delta.
		if _, err := tx.Exec(ctx,
			`INSERT INTO stok (gudang_id, produk_id, qty, updated_at)
			 VALUES ($1, $2, $3, now())
			 ON CONFLICT (gudang_id, produk_id)
			 DO UPDATE SET qty = stok.qty + EXCLUDED.qty, updated_at = now()`,
			in.GudangID, in.ProdukID, qtyKonversi,
		); err != nil {
			return fmt.Errorf("upsert stok: %w", err)
		}

		// Insert baris adjustment.
		if err := s.repo.Create(ctx, tx, a); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Audit log eksternal (opsional). Tidak fatal jika gagal — sudah ada
	// stok_adjustment row sebagai sumber kebenaran audit utk modul ini.
	if s.audit != nil {
		uid := userID
		_ = s.audit.Record(ctx, RecordEntry{
			UserID:   &uid,
			Aksi:     "create",
			Tabel:    "stok_adjustment",
			RecordID: a.ID,
			After: map[string]any{
				"gudang_id":    a.GudangID,
				"produk_id":    a.ProdukID,
				"satuan_id":    a.SatuanID,
				"qty":          a.Qty,
				"qty_konversi": a.QtyKonversi,
				"kategori":     a.Kategori,
				"alasan":       a.Alasan,
				"catatan":      a.Catatan,
			},
		})
	}

	return a, nil
}

// Get satu adjustment.
func (s *StokAdjustmentService) Get(ctx context.Context, id int64) (*domain.StokAdjustment, error) {
	return s.repo.Get(ctx, id)
}

// List paginated.
func (s *StokAdjustmentService) List(
	ctx context.Context, f repo.ListAdjFilter,
) (PageResult[domain.StokAdjustment], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.StokAdjustment]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

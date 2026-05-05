//go:build integration
// +build integration

// Integration tests tambahan untuk menaikkan coverage repo dari 0% baseline.
// Reuse fixture helpers (newFixture, openTestPool, seedStok) dari
// penjualan_concurrent_test.go.
//
// Run:
//   go test -tags=integration -timeout 60s -cover ./internal/repo/ -count=1
//
// Mensyaratkan PostgreSQL @ localhost:5544 (docker-compose up db) atau
// env DB_TEST_URL. Test akan SKIP kalau koneksi gagal.

package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// TestGudangRepo - List + GetByID + GetByKode.
func TestGudangRepo(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	r := NewGudangRepo(pool)

	// List all (include inactive).
	all, err := r.List(ctx, true)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) < 3 {
		t.Errorf("expected >=3 gudang (fixture buat 3), got %d", len(all))
	}

	// GetByID untuk gudang fixture.
	g, err := r.GetByID(ctx, f.gudangAID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if g.ID != f.gudangAID {
		t.Errorf("ID mismatch: got %d, want %d", g.ID, f.gudangAID)
	}

	// GetByKode (kode dibuat fixture: GA_<short>).
	if _, err := r.GetByKode(ctx, g.Kode); err != nil {
		t.Errorf("GetByKode: %v", err)
	}

	// GetByID not-found → sentinel error.
	_, err = r.GetByID(ctx, -999)
	if !errors.Is(err, domain.ErrGudangNotFound) {
		t.Errorf("expected ErrGudangNotFound, got %v", err)
	}

	// List active-only juga.
	if _, err := r.List(ctx, false); err != nil {
		t.Errorf("List active-only: %v", err)
	}
}

// TestSatuanRepo - List + GetByID + GetByKode.
func TestSatuanRepo(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	r := NewSatuanRepo(pool)

	if _, err := r.List(ctx); err != nil {
		t.Fatalf("List: %v", err)
	}
	s, err := r.GetByID(ctx, f.satuanID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if s.ID != f.satuanID {
		t.Errorf("id mismatch")
	}
	if _, err := r.GetByKode(ctx, s.Kode); err != nil {
		t.Errorf("GetByKode: %v", err)
	}
	if _, err := r.GetByID(ctx, -1); !errors.Is(err, domain.ErrSatuanNotFound) {
		t.Errorf("expected ErrSatuanNotFound, got %v", err)
	}
}

// TestProdukRepo - GetByID, GetBySKU, Search, List, ListKategori.
func TestProdukRepo(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	r := NewProdukRepo(pool)

	p, err := r.GetByID(ctx, f.produkID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if p.ID != f.produkID {
		t.Errorf("id mismatch")
	}
	if _, err := r.GetBySKU(ctx, p.SKU); err != nil {
		t.Errorf("GetBySKU: %v", err)
	}

	if _, err := r.Search(ctx, p.Nama, 10); err != nil {
		t.Errorf("Search: %v", err)
	}

	// List default (page 1, perpage 25 default).
	if _, _, err := r.List(ctx, ListProdukFilter{}); err != nil {
		t.Errorf("List: %v", err)
	}
	if _, err := r.ListKategori(ctx); err != nil {
		t.Errorf("ListKategori: %v", err)
	}

	if _, err := r.GetByID(ctx, -123); !errors.Is(err, domain.ErrProdukNotFound) {
		t.Errorf("expected ErrProdukNotFound, got %v", err)
	}
}

// TestPenjualanRepoCRUD - Create + GetByID + GetByNomor + GetByClientUUID +
// LoadItems + List + ListWithRelations + NextNomor + StokInfoOf + HasPembayaran +
// SearchByNomor.
func TestPenjualanRepoCRUD(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	seedStok(t, pool, f.gudangAID, f.produkID, 50)

	r := NewPenjualanRepo(pool)
	tanggal := time.Now().Truncate(24 * time.Hour)
	uid := uuid.New()
	nomor := "INV/CRUD/" + f.suffix

	p := &domain.Penjualan{
		NomorKwitansi: nomor,
		Tanggal:       tanggal,
		MitraID:       f.mitraID,
		GudangID:      f.gudangAID,
		UserID:        f.userID,
		Subtotal:      100000,
		DPP:           100000,
		Total:         100000,
		StatusBayar:   domain.StatusLunas,
		ClientUUID:    uid,
		Items: []domain.PenjualanItem{{
			ProdukID: f.produkID, ProdukNama: "p", Qty: 2,
			SatuanID: f.satuanID, SatuanKode: "x", QtyKonversi: 2,
			HargaSatuan: 50000, Subtotal: 100000,
		}},
	}
	if err := r.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("ID not set after Create")
	}

	// GetByID
	got, err := r.GetByID(ctx, p.ID, &p.Tanggal)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.NomorKwitansi != nomor {
		t.Errorf("nomor mismatch")
	}

	// GetByID without tanggal pruning.
	if _, err := r.GetByID(ctx, p.ID, nil); err != nil {
		t.Errorf("GetByID no tanggal: %v", err)
	}

	// GetByNomor
	if _, err := r.GetByNomor(ctx, nomor); err != nil {
		t.Errorf("GetByNomor: %v", err)
	}
	// GetByNomor not found.
	if _, err := r.GetByNomor(ctx, "zzz/notexist/123"); !errors.Is(err, domain.ErrPenjualanNotFound) {
		t.Errorf("expected ErrPenjualanNotFound, got %v", err)
	}

	// GetByClientUUID + idempotency check.
	if _, err := r.GetByClientUUID(ctx, uid); err != nil {
		t.Errorf("GetByClientUUID: %v", err)
	}
	if _, err := r.GetByClientUUID(ctx, uuid.New()); !errors.Is(err, domain.ErrPenjualanNotFound) {
		t.Errorf("expected ErrPenjualanNotFound for random uuid, got %v", err)
	}

	// LoadItems
	if err := r.LoadItems(ctx, got); err != nil {
		t.Errorf("LoadItems: %v", err)
	}
	if len(got.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(got.Items))
	}

	// List filter by gudang.
	gid := f.gudangAID
	listed, total, err := r.List(ctx, ListPenjualanFilter{GudangID: &gid})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 1 || len(listed) < 1 {
		t.Errorf("expected at least 1 in list")
	}

	// ListWithRelations
	if _, _, err := r.ListWithRelations(ctx, ListPenjualanFilter{GudangID: &gid}); err != nil {
		t.Errorf("ListWithRelations: %v", err)
	}

	// NextNomor
	if _, err := r.NextNomor(ctx, "GA", tanggal); err != nil {
		t.Errorf("NextNomor: %v", err)
	}

	// StokInfoOf
	si, err := r.StokInfoOf(ctx, f.gudangAID, f.produkID)
	if err != nil {
		t.Errorf("StokInfoOf: %v", err)
	}
	if si.Qty < 0 {
		t.Errorf("stok negatif: %v", si.Qty)
	}

	// SearchByNomor
	if _, err := r.SearchByNomor(ctx, "INV/CRUD", 5); err != nil {
		t.Errorf("SearchByNomor: %v", err)
	}
	if got, err := r.SearchByNomor(ctx, "  ", 5); err != nil || got != nil {
		t.Errorf("SearchByNomor empty query should return nil")
	}

	// HasPembayaran (belum ada bayar → false).
	exists, err := r.HasPembayaran(ctx, p.ID, p.Tanggal)
	if err != nil {
		t.Errorf("HasPembayaran: %v", err)
	}
	if exists {
		t.Errorf("expected no pembayaran")
	}
}

// TestPembayaranRepoCRUD - Create + GetByID + GetByClientUUID + ListByMitra +
// ListByPenjualan + SumByPenjualan + SumByMitra + idempotency duplicate uuid.
func TestPembayaranRepoCRUD(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	seedStok(t, pool, f.gudangAID, f.produkID, 50)

	// Setup: 1 invoice kredit utk dibayar.
	pjr := NewPenjualanRepo(pool)
	tanggal := time.Now().Truncate(24 * time.Hour)
	jt := tanggal.AddDate(0, 1, 0)
	pj := &domain.Penjualan{
		NomorKwitansi: "INV/PAY/" + f.suffix,
		Tanggal:       tanggal,
		MitraID:       f.mitraID,
		GudangID:      f.gudangAID,
		UserID:        f.userID,
		Subtotal:      100000,
		DPP:           100000,
		Total:         100000,
		StatusBayar:   domain.StatusKredit,
		JatuhTempo:    &jt,
		ClientUUID:    uuid.New(),
		Items: []domain.PenjualanItem{{
			ProdukID: f.produkID, ProdukNama: "p", Qty: 1,
			SatuanID: f.satuanID, SatuanKode: "x", QtyKonversi: 1,
			HargaSatuan: 100000, Subtotal: 100000,
		}},
	}
	if err := pjr.Create(ctx, pj); err != nil {
		t.Fatalf("Create invoice: %v", err)
	}

	r := NewPembayaranRepo(pool)
	uid := uuid.New()
	pay := &domain.Pembayaran{
		PenjualanID:      &pj.ID,
		PenjualanTanggal: &pj.Tanggal,
		MitraID:          f.mitraID,
		Tanggal:          tanggal,
		Jumlah:           50000,
		Metode:           domain.MetodeTunai,
		UserID:           f.userID,
		ClientUUID:       uid,
	}
	if err := r.Create(ctx, pay); err != nil {
		t.Fatalf("Create pembayaran: %v", err)
	}
	if pay.ID == 0 {
		t.Fatal("pembayaran ID not set")
	}

	// GetByID
	if _, err := r.GetByID(ctx, pay.ID); err != nil {
		t.Errorf("GetByID: %v", err)
	}
	if _, err := r.GetByID(ctx, -1); !errors.Is(err, domain.ErrPembayaranNotFound) {
		t.Errorf("expected ErrPembayaranNotFound, got %v", err)
	}

	// GetByClientUUID
	if _, err := r.GetByClientUUID(ctx, uid); err != nil {
		t.Errorf("GetByClientUUID: %v", err)
	}
	if _, err := r.GetByClientUUID(ctx, uuid.New()); !errors.Is(err, domain.ErrPembayaranNotFound) {
		t.Errorf("expected ErrPembayaranNotFound, got %v", err)
	}

	// Idempotency: insert dgn client_uuid sama → unique violation (23505).
	dup := *pay
	dup.ID = 0
	if err := r.Create(ctx, &dup); err == nil {
		t.Errorf("expected duplicate uuid error")
	}

	// ListByMitra
	listed, total, err := r.ListByMitra(ctx, f.mitraID, ListPembayaranFilter{})
	if err != nil {
		t.Errorf("ListByMitra: %v", err)
	}
	if total < 1 || len(listed) < 1 {
		t.Errorf("expected at least 1 pembayaran")
	}

	// ListByPenjualan
	got, err := r.ListByPenjualan(ctx, pj.ID, pj.Tanggal)
	if err != nil {
		t.Errorf("ListByPenjualan: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 pembayaran, got %d", len(got))
	}

	// SumByPenjualan
	sum, err := r.SumByPenjualan(ctx, pj.ID, pj.Tanggal)
	if err != nil {
		t.Errorf("SumByPenjualan: %v", err)
	}
	if sum != 50000 {
		t.Errorf("expected sum=50000, got %d", sum)
	}

	// SumByMitra (without/with until).
	if _, err := r.SumByMitra(ctx, f.mitraID, time.Time{}); err != nil {
		t.Errorf("SumByMitra unbounded: %v", err)
	}
	if _, err := r.SumByMitra(ctx, f.mitraID, time.Now()); err != nil {
		t.Errorf("SumByMitra until now: %v", err)
	}

	// HasPembayaran sekarang true.
	exists, err := pjr.HasPembayaran(ctx, pj.ID, pj.Tanggal)
	if err != nil {
		t.Errorf("HasPembayaran: %v", err)
	}
	if !exists {
		t.Errorf("expected pembayaran exists")
	}
}

// TestAuditLogRepo - Create + GetByID + List + ListTabel.
func TestAuditLogRepo(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	r := NewAuditLogRepo(pool)
	uid := f.userID
	l := &domain.AuditLog{
		UserID:   &uid,
		Aksi:     "create",
		Tabel:    "test_audit_" + f.suffix,
		RecordID: 42,
	}
	if err := r.Create(ctx, l); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if l.ID == 0 {
		t.Fatal("ID not set")
	}

	got, err := r.GetByID(ctx, l.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Aksi != "create" {
		t.Errorf("aksi mismatch")
	}

	if _, err := r.GetByID(ctx, -1); !errors.Is(err, ErrAuditLogNotFound) {
		t.Errorf("expected ErrAuditLogNotFound, got %v", err)
	}

	tabel := l.Tabel
	if _, _, err := r.List(ctx, ListAuditFilter{Tabel: &tabel}); err != nil {
		t.Errorf("List: %v", err)
	}
	if _, err := r.ListTabel(ctx); err != nil {
		t.Errorf("ListTabel: %v", err)
	}

	// Cleanup: hapus audit row supaya tidak menumpuk.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE id = $1`, l.ID)
	})
}

// TestAdjRepo - Create dlm tx + Get + List.
func TestAdjRepo(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)
	ctx := context.Background()

	r := NewAdjRepo(pool)

	a := &domain.StokAdjustment{
		GudangID:    f.gudangAID,
		ProdukID:    f.produkID,
		SatuanID:    f.satuanID,
		Qty:         5,
		QtyKonversi: 5,
		Kategori:    domain.AdjKategoriInitial,
		Alasan:      domain.AdjAlasanDefault(domain.AdjKategoriInitial),
		UserID:      f.userID,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := r.Create(ctx, tx, a); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("Create: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got, err := r.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GudangID != f.gudangAID || got.ProdukID != f.produkID {
		t.Errorf("relation mismatch")
	}

	if _, err := r.Get(ctx, -1); !errors.Is(err, domain.ErrAdjTidakDitemukan) {
		t.Errorf("expected ErrAdjTidakDitemukan, got %v", err)
	}

	gid := f.gudangAID
	if _, _, err := r.List(ctx, ListAdjFilter{GudangID: &gid}); err != nil {
		t.Errorf("List: %v", err)
	}
}

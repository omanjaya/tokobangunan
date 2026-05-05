//go:build integration
// +build integration

// Integration tests untuk service layer. Reuse pola fixture dari
// internal/repo/penjualan_concurrent_test.go: bikin gudang/produk/satuan/mitra
// random per test, cleanup di t.Cleanup.
//
// Run:
//   go test -tags=integration -timeout 60s -cover ./internal/service/ -count=1
//
// Mensyaratkan PostgreSQL @ localhost:5544 atau env DB_TEST_URL.

package service

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

const defaultTestDSN = "postgres://dev:dev@localhost:5544/tokobangunan?sslmode=disable"

func testDSN() string {
	if v := os.Getenv("DB_TEST_URL"); v != "" {
		return v
	}
	return defaultTestDSN
}

func randAlpha(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, testDSN())
	if err != nil {
		t.Skipf("skip: cannot init pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skip: cannot ping db (%s): %v", testDSN(), err)
	}
	return pool
}

type svcFixture struct {
	gudangID int64
	satuanID int64
	produkID int64
	userID   int64
	suffix   string
}

func newSvcFixture(t *testing.T, pool *pgxpool.Pool) *svcFixture {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("ST%d_%d", time.Now().UnixNano(), rand.Intn(1<<16))
	short := suffix
	if len(short) > 12 {
		short = short[len(short)-12:]
	}
	f := &svcFixture{suffix: suffix}

	// User existing id=1 atau bikin baru.
	if err := pool.QueryRow(ctx, `SELECT id FROM "user" WHERE id = 1`).Scan(&f.userID); err != nil {
		err2 := pool.QueryRow(ctx, `
			INSERT INTO "user" (username, password_hash, nama_lengkap, role)
			VALUES ($1, '$2y$10$xxxxxxxxxxxxxxxxxxxxxx', 'Test', 'admin')
			RETURNING id`, "u_"+short).Scan(&f.userID)
		if err2 != nil {
			t.Fatalf("user: %v", err2)
		}
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO satuan (kode, nama) VALUES ($1, $2) RETURNING id`,
		"st_"+short, "Sat "+short).Scan(&f.satuanID); err != nil {
		t.Fatalf("satuan: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO gudang (kode, nama) VALUES ($1, $2) RETURNING id`,
		"GS_"+short, "Gudang "+short).Scan(&f.gudangID); err != nil {
		t.Fatalf("gudang: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO produk (sku, nama, satuan_kecil_id, faktor_konversi)
		 VALUES ($1, $2, $3, 1) RETURNING id`,
		"sku_"+short, "Produk "+short, f.satuanID).Scan(&f.produkID); err != nil {
		t.Fatalf("produk: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM stok_adjustment WHERE produk_id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM stok WHERE produk_id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM produk WHERE id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM gudang WHERE id = $1`, f.gudangID)
		_, _ = pool.Exec(ctx, `DELETE FROM satuan WHERE id = $1`, f.satuanID)
	})
	return f
}

// TestGudangService - List + Get + Create + Update + SetActive (wrapper).
func TestGudangService(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newSvcFixture(t, pool)
	ctx := context.Background()

	s := NewGudangService(repo.NewGudangRepo(pool))

	// List include inactive.
	all, err := s.List(ctx, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) < 1 {
		t.Errorf("expected >=1 gudang")
	}

	// Get fixture.
	if _, err := s.Get(ctx, f.gudangID); err != nil {
		t.Errorf("Get: %v", err)
	}

	// Create new gudang.
	// Kode harus A-Z + underscore. Pakai random alpha suffix.
	alpha := randAlpha(6)
	created, err := s.Create(ctx, dto.GudangCreateInput{
		Kode: "NEW_" + alpha, Nama: "New", IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("ID not set")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM gudang WHERE id = $1`, created.ID)
	})

	// Update.
	updated, err := s.Update(ctx, created.ID, dto.GudangUpdateInput{
		Kode: created.Kode, Nama: "Updated", IsActive: true,
	})
	if err != nil {
		t.Errorf("Update: %v", err)
	}
	if updated.Nama != "Updated" {
		t.Errorf("expected nama=Updated, got %s", updated.Nama)
	}

	// Duplicate kode → ErrGudangKodeDuplikat.
	_, err = s.Create(ctx, dto.GudangCreateInput{
		Kode: created.Kode, Nama: "dup", IsActive: true,
	})
	if err == nil {
		t.Errorf("expected duplicate error")
	}

	// SetActive false (soft delete).
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

// TestSatuanService - List + Get + Create + duplicate kode.
func TestSatuanService(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newSvcFixture(t, pool)
	ctx := context.Background()

	s := NewSatuanService(repo.NewSatuanRepo(pool))

	if _, err := s.List(ctx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := s.Get(ctx, f.satuanID); err != nil {
		t.Errorf("Get: %v", err)
	}

	short := f.suffix
	if len(short) > 8 {
		short = short[len(short)-8:]
	}
	created, err := s.Create(ctx, dto.SatuanCreateInput{
		Kode: "ns_" + short, Nama: "New Sat",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM satuan WHERE id = $1`, created.ID)
	})

	// Duplicate.
	_, err = s.Create(ctx, dto.SatuanCreateInput{
		Kode: created.Kode, Nama: "dup",
	})
	if err == nil {
		t.Errorf("expected duplicate err")
	}
}

// TestStokAdjustmentService_Create - end-to-end Create flow:
// validate → resolve produk/satuan → upsert stok → insert adjustment row →
// audit log. Reuse fixture, gudang fresh.
func TestStokAdjustmentService_Create(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newSvcFixture(t, pool)
	ctx := context.Background()

	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	adjRepo := repo.NewAdjRepo(pool)
	auditSvc := NewAuditLogService(repo.NewAuditLogRepo(pool))

	svc := NewStokAdjustmentService(pool, adjRepo, produkRepo, satuanRepo, auditSvc)

	// 1. Create positive adjustment (initial).
	in := dto.StokAdjustmentInput{
		GudangID: f.gudangID,
		ProdukID: f.produkID,
		SatuanID: f.satuanID,
		Qty:      10,
		Kategori: domain.AdjKategoriInitial,
		Catatan:  "stok awal",
	}
	a, err := svc.Create(ctx, f.userID, in)
	if err != nil {
		t.Fatalf("Create positive: %v", err)
	}
	if a.ID == 0 {
		t.Fatal("ID not set")
	}
	if a.QtyKonversi != 10 {
		t.Errorf("qty_konversi expected 10, got %v", a.QtyKonversi)
	}

	// Verify stok upserted.
	var qty float64
	if err := pool.QueryRow(ctx,
		`SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2`,
		f.gudangID, f.produkID).Scan(&qty); err != nil {
		t.Fatalf("query stok: %v", err)
	}
	if qty != 10 {
		t.Errorf("expected stok=10, got %v", qty)
	}

	// 2. Get + List.
	if _, err := svc.Get(ctx, a.ID); err != nil {
		t.Errorf("Get: %v", err)
	}
	gid := f.gudangID
	if _, err := svc.List(ctx, repo.ListAdjFilter{GudangID: &gid}); err != nil {
		t.Errorf("List: %v", err)
	}

	// 3. Negative adjustment that fits.
	inNeg := in
	inNeg.Qty = -3
	inNeg.Kategori = domain.AdjKategoriRusak
	if _, err := svc.Create(ctx, f.userID, inNeg); err != nil {
		t.Errorf("Create negative: %v", err)
	}
	_ = pool.QueryRow(ctx,
		`SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2`,
		f.gudangID, f.produkID).Scan(&qty)
	if qty != 7 {
		t.Errorf("expected stok=7 after -3, got %v", qty)
	}

	// 4. Negative adjustment that exceeds → ErrAdjStokTidakCukup.
	inOver := in
	inOver.Qty = -100
	inOver.Kategori = domain.AdjKategoriRusak
	if _, err := svc.Create(ctx, f.userID, inOver); err == nil {
		t.Errorf("expected ErrAdjStokTidakCukup")
	}

	// 5. Validation: qty=0 → ErrAdjQtyNol.
	bad := in
	bad.Qty = 0
	if _, err := svc.Create(ctx, f.userID, bad); err == nil {
		t.Errorf("expected validation err qty=0")
	}

	// 6. userID 0 ditolak.
	if _, err := svc.Create(ctx, 0, in); err == nil {
		t.Errorf("expected error for userID=0")
	}

	// 7. Kategori invalid → validate fail.
	badK := in
	badK.Kategori = "bogus"
	if _, err := svc.Create(ctx, f.userID, badK); err == nil {
		t.Errorf("expected kategori invalid err")
	}
}

// TestProdukService - List + Get + Search + Create + Update + Delete + ListKategori.
func TestProdukService(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newSvcFixture(t, pool)
	ctx := context.Background()

	svc := NewProdukService(repo.NewProdukRepo(pool), repo.NewSatuanRepo(pool))

	// Wrappers.
	if _, err := svc.List(ctx, repo.ListProdukFilter{}); err != nil {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Get(ctx, f.produkID); err != nil {
		t.Errorf("Get: %v", err)
	}
	if _, err := svc.Search(ctx, "Produk", 5); err != nil {
		t.Errorf("Search: %v", err)
	}
	if _, err := svc.ListKategori(ctx); err != nil {
		t.Errorf("ListKategori: %v", err)
	}

	// Create new produk.
	short := f.suffix
	if len(short) > 8 {
		short = short[len(short)-8:]
	}
	created, err := svc.Create(ctx, dto.ProdukCreateInput{
		SKU:            "newsku_" + short,
		Nama:           "Baru " + short,
		SatuanKecilID:  f.satuanID,
		FaktorKonversi: 1,
		IsActive:       true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM produk WHERE id = $1`, created.ID)
	})

	// Update.
	if _, err := svc.Update(ctx, created.ID, dto.ProdukUpdateInput{
		SKU: created.SKU, Nama: "Updated " + short,
		SatuanKecilID: f.satuanID, FaktorKonversi: 1, IsActive: true,
	}); err != nil {
		t.Errorf("Update: %v", err)
	}

	// Duplicate SKU error.
	if _, err := svc.Create(ctx, dto.ProdukCreateInput{
		SKU: created.SKU, Nama: "dup",
		SatuanKecilID: f.satuanID, FaktorKonversi: 1, IsActive: true,
	}); err == nil {
		t.Errorf("expected dupe SKU error")
	}

	// Validation: SKU empty.
	if _, err := svc.Create(ctx, dto.ProdukCreateInput{
		SKU: "", Nama: "x", SatuanKecilID: f.satuanID, FaktorKonversi: 1,
	}); err == nil {
		t.Errorf("expected validation err")
	}

	// SetFotoURL.
	url := "/foo.jpg"
	if err := svc.SetFotoURL(ctx, created.ID, &url); err != nil {
		t.Errorf("SetFotoURL: %v", err)
	}

	// Delete soft.
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

// TestMitraService - Create + Get + List + Update + Search + Delete.
func TestMitraService(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	_ = newSvcFixture(t, pool) // bikin user/satuan/dll, plus cleanup
	ctx := context.Background()

	svc := NewMitraService(repo.NewMitraRepo(pool))

	short := randAlpha(6)
	created, err := svc.Create(ctx, CreateMitraInput{
		Kode:     "MS_" + short,
		Nama:     "Mitra " + short,
		Tipe:     "eceran",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM mitra WHERE id = $1`, created.ID)
	})

	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Errorf("Get: %v", err)
	}
	if _, err := svc.List(ctx, repo.ListMitraFilter{}); err != nil {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Search(ctx, "Mitra", 5); err != nil {
		t.Errorf("Search: %v", err)
	}

	updated, err := svc.Update(ctx, UpdateMitraInput{
		ID:       created.ID,
		Kode:     created.Kode,
		Nama:     "Updated",
		Tipe:     "eceran",
		IsActive: true,
		Version:  created.Version,
	})
	if err != nil {
		t.Errorf("Update: %v", err)
	}
	if updated != nil && updated.Nama != "Updated" {
		t.Errorf("nama mismatch: %s", updated.Nama)
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

// TestAuditLogService - Record + List + Get + ListTabel + WithAuditUser ctx.
func TestAuditLogService(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newSvcFixture(t, pool)
	ctx := context.Background()

	svc := NewAuditLogService(repo.NewAuditLogRepo(pool))

	uid := f.userID
	tabel := "test_svc_audit_" + f.suffix
	if err := svc.Record(ctx, RecordEntry{
		UserID:   &uid,
		Aksi:     "create",
		Tabel:    tabel,
		RecordID: 99,
		After:    map[string]any{"x": 1},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	page, err := svc.List(ctx, repo.ListAuditFilter{Tabel: &tabel})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total < 1 || len(page.Items) < 1 {
		t.Errorf("expected at least 1 audit entry")
	}

	if _, err := svc.Get(ctx, page.Items[0].ID); err != nil {
		t.Errorf("Get: %v", err)
	}

	if _, err := svc.ListTabel(ctx); err != nil {
		t.Errorf("ListTabel: %v", err)
	}

	// Context user helpers.
	c2 := WithAuditUser(ctx, 42)
	if v := AuditUserFromContext(c2); v != 42 {
		t.Errorf("AuditUserFromContext expected 42, got %d", v)
	}
	if v := AuditUserFromContext(ctx); v != 0 {
		t.Errorf("expected 0 for unset, got %d", v)
	}

	// Cleanup audit rows.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE tabel = $1`, tabel)
	})
}

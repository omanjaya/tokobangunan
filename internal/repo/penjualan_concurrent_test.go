//go:build integration
// +build integration

// Concurrent stress tests untuk hot paths (penjualan, pembayaran, mutasi).
//
// Run:
//   go test -race -tags=integration -timeout 60s ./internal/repo/ \
//       -run TestStokConcurrent -v
//   go test -race -tags=integration -timeout 60s ./internal/repo/ \
//       -run TestPembayaranConcurrent -v
//   go test -race -tags=integration -timeout 60s ./internal/repo/ \
//       -run TestMutasiConcurrent -v
//
// Requires PostgreSQL @ localhost:5544 (docker-compose up db) atau
// env DB_TEST_URL untuk override. Test akan SKIP kalau koneksi gagal.
//
// Setiap test bikin fixture sendiri (gudang/produk/satuan/mitra/user)
// dengan kode random supaya tidak bentrok dengan data dev. Cleanup pada
// t.Cleanup menghapus row yang dibuat (best-effort).

package repo

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

const defaultTestDSN = "postgres://dev:dev@localhost:5544/tokobangunan?sslmode=disable"

func testDSN() string {
	if v := os.Getenv("DB_TEST_URL"); v != "" {
		return v
	}
	return defaultTestDSN
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

type fixtureIDs struct {
	gudangAID int64
	gudangBID int64
	gudangCID int64
	satuanID  int64
	produkID  int64
	mitraID   int64
	userID    int64
	suffix    string
}

func newFixture(t *testing.T, pool *pgxpool.Pool) *fixtureIDs {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("RT%d_%d", time.Now().UnixNano(), rand.Intn(1<<16))
	short := suffix
	if len(short) > 12 {
		short = short[len(short)-12:]
	}
	f := &fixtureIDs{suffix: suffix}

	// Use existing user (id=1, owner) to satisfy any FK; create-our-own else.
	if err := pool.QueryRow(ctx, `SELECT id FROM "user" WHERE id = 1`).Scan(&f.userID); err != nil {
		// Create one if id=1 not exist.
		err2 := pool.QueryRow(ctx, `
			INSERT INTO "user" (username, password_hash, nama_lengkap, role)
			VALUES ($1, '$2y$10$xxxxxxxxxxxxxxxxxxxxxx', 'Test User', 'admin')
			RETURNING id`, "u_"+short).Scan(&f.userID)
		if err2 != nil {
			t.Fatalf("create user: %v", err2)
		}
	}

	// Satuan
	if err := pool.QueryRow(ctx, `
		INSERT INTO satuan (kode, nama) VALUES ($1, $2) RETURNING id`,
		"st_"+short, "Sat "+short).Scan(&f.satuanID); err != nil {
		t.Fatalf("create satuan: %v", err)
	}

	// 3 gudang
	for _, gPtr := range []struct {
		out *int64
		k   string
	}{{&f.gudangAID, "GA_" + short}, {&f.gudangBID, "GB_" + short}, {&f.gudangCID, "GC_" + short}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO gudang (kode, nama) VALUES ($1, $2) RETURNING id`,
			gPtr.k, "Gudang "+gPtr.k).Scan(gPtr.out); err != nil {
			t.Fatalf("create gudang %s: %v", gPtr.k, err)
		}
	}

	// Produk
	if err := pool.QueryRow(ctx, `
		INSERT INTO produk (sku, nama, satuan_kecil_id, faktor_konversi)
		VALUES ($1, $2, $3, 1) RETURNING id`,
		"sku_"+short, "Produk "+short, f.satuanID).Scan(&f.produkID); err != nil {
		t.Fatalf("create produk: %v", err)
	}

	// Mitra
	if err := pool.QueryRow(ctx, `
		INSERT INTO mitra (kode, nama, tipe, limit_kredit)
		VALUES ($1, $2, 'customer', 100000000) RETURNING id`,
		"M_"+short, "Mitra "+short).Scan(&f.mitraID); err != nil {
		t.Fatalf("create mitra: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		// Delete order matters due to FK.
		_, _ = pool.Exec(ctx, `DELETE FROM pembayaran WHERE mitra_id = $1`, f.mitraID)
		_, _ = pool.Exec(ctx, `DELETE FROM penjualan_item WHERE produk_id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM penjualan WHERE mitra_id = $1`, f.mitraID)
		_, _ = pool.Exec(ctx, `DELETE FROM mutasi_item WHERE produk_id = $1`, f.produkID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM mutasi_gudang WHERE gudang_asal_id IN ($1,$2,$3)`,
			f.gudangAID, f.gudangBID, f.gudangCID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM stok WHERE produk_id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM produk WHERE id = $1`, f.produkID)
		_, _ = pool.Exec(ctx, `DELETE FROM mitra WHERE id = $1`, f.mitraID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM gudang WHERE id IN ($1,$2,$3)`,
			f.gudangAID, f.gudangBID, f.gudangCID)
		_, _ = pool.Exec(ctx, `DELETE FROM satuan WHERE id = $1`, f.satuanID)
	})
	return f
}

// seed stok row with given qty in gudang.
func seedStok(t *testing.T, pool *pgxpool.Pool, gudangID, produkID int64, qty float64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO stok (gudang_id, produk_id, qty) VALUES ($1,$2,$3)
		 ON CONFLICT (gudang_id, produk_id) DO UPDATE SET qty = EXCLUDED.qty`,
		gudangID, produkID, qty)
	if err != nil {
		t.Fatalf("seed stok: %v", err)
	}
}

func getStok(t *testing.T, pool *pgxpool.Pool, gudangID, produkID int64) float64 {
	t.Helper()
	var q float64
	err := pool.QueryRow(context.Background(),
		`SELECT qty FROM stok WHERE gudang_id=$1 AND produk_id=$2`,
		gudangID, produkID).Scan(&q)
	if err == pgx.ErrNoRows {
		return 0
	}
	if err != nil {
		t.Fatalf("get stok: %v", err)
	}
	return q
}

// TestStokConcurrent — 50 goroutine concurrent penjualan repo.Create
// untuk produk/gudang yg sama. Initial stok=100, qty per req=2.
// Expected: tepat 50 sukses, stok akhir=0, no oversold.
func TestStokConcurrent(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)

	const initial = 100.0
	const qtyPer = 2.0
	const concurrency = 50

	seedStok(t, pool, f.gudangAID, f.produkID, initial)
	r := NewPenjualanRepo(pool)
	tanggal := time.Now().Truncate(24 * time.Hour)

	var ok atomic.Int32
	var fail atomic.Int32
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			p := &domain.Penjualan{
				NomorKwitansi: fmt.Sprintf("INV/%s/%04d", f.suffix, i),
				Tanggal:       tanggal,
				MitraID:       f.mitraID,
				GudangID:      f.gudangAID,
				UserID:        f.userID,
				Subtotal:      10000,
				Diskon:        0,
				DPP:           10000,
				PPNPersen:     0,
				PPNAmount:     0,
				Total:         10000,
				StatusBayar:   domain.StatusLunas,
				ClientUUID:    uuid.New(),
				Items: []domain.PenjualanItem{{
					ProdukID:    f.produkID,
					ProdukNama:  "p",
					Qty:         qtyPer,
					SatuanID:    f.satuanID,
					SatuanKode:  "x",
					QtyKonversi: qtyPer,
					HargaSatuan: 5000,
					Subtotal:    10000,
				}},
			}
			if err := r.Create(ctx, p); err != nil {
				fail.Add(1)
				if !strings.Contains(err.Error(), "tidak cukup") {
					t.Logf("non-stock err: %v", err)
				}
				return
			}
			ok.Add(1)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	final := getStok(t, pool, f.gudangAID, f.produkID)
	expectedSucc := int(initial / qtyPer) // 50
	t.Logf("concurrency=%d ok=%d fail=%d elapsed=%v throughput=%.1f req/s final_stok=%.4f",
		concurrency, ok.Load(), fail.Load(), elapsed,
		float64(concurrency)/elapsed.Seconds(), final)
	if int(ok.Load()) != expectedSucc {
		t.Errorf("expected ok=%d got %d (fail=%d)", expectedSucc, ok.Load(), fail.Load())
	}
	if final != 0 {
		t.Errorf("expected final stok=0, got %.4f (oversold/underused)", final)
	}
	if final < 0 {
		t.Fatalf("STOK NEGATIF! oversold detected: %.4f", final)
	}
}

// TestPembayaranConcurrent — 5 goroutine bayar 200rb each ke invoice 1jt.
// Expected: SUM=1jt, status=lunas, no over.
// Bonus: 5 goroutine bayar 300rb each ke invoice 1jt baru.
// Expected: total approved <= 1jt (kelebihan ditolak).
func TestPembayaranConcurrent(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)

	seedStok(t, pool, f.gudangAID, f.produkID, 100)
	tanggal := time.Now().Truncate(24 * time.Hour)
	totalCents := int64(100_000_000) // 1jt rupiah * 100 cents

	mkInvoice := func(suffix string) (int64, time.Time) {
		t.Helper()
		r := NewPenjualanRepo(pool)
		jt := tanggal.AddDate(0, 1, 0)
		p := &domain.Penjualan{
			NomorKwitansi: fmt.Sprintf("INV/%s/%s", f.suffix, suffix),
			Tanggal:       tanggal,
			MitraID:       f.mitraID,
			GudangID:      f.gudangAID,
			UserID:        f.userID,
			Subtotal:      totalCents,
			DPP:           totalCents,
			Total:         totalCents,
			StatusBayar:   domain.StatusKredit,
			JatuhTempo:    &jt,
			ClientUUID:    uuid.New(),
			Items: []domain.PenjualanItem{{
				ProdukID: f.produkID, ProdukNama: "p",
				Qty: 1, SatuanID: f.satuanID, SatuanKode: "x",
				QtyKonversi: 1, HargaSatuan: totalCents, Subtotal: totalCents,
			}},
		}
		if err := r.Create(context.Background(), p); err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		return p.ID, p.Tanggal
	}

	pembRepo := NewPembayaranRepo(pool)

	// Helper: simulate service-level lock (mirip PembayaranService.Record).
	pay := func(invID int64, invTgl time.Time, amount int64) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			var total int64
			if err := tx.QueryRow(ctx,
				`SELECT total FROM penjualan WHERE id=$1 AND tanggal=$2 FOR UPDATE`,
				invID, invTgl).Scan(&total); err != nil {
				return err
			}
			var dibayar int64
			if err := tx.QueryRow(ctx,
				`SELECT COALESCE(SUM(jumlah),0) FROM pembayaran
				 WHERE penjualan_id=$1 AND penjualan_tanggal=$2`,
				invID, invTgl).Scan(&dibayar); err != nil {
				return err
			}
			outs := total - dibayar
			if amount > outs {
				return domain.ErrJumlahLebihDariOutstanding
			}
			p := &domain.Pembayaran{
				PenjualanID:      &invID,
				PenjualanTanggal: &invTgl,
				MitraID:          f.mitraID,
				Tanggal:          invTgl,
				Jumlah:           amount,
				Metode:           domain.MetodeTunai,
				UserID:           f.userID,
				ClientUUID:       uuid.New(),
			}
			return pembRepo.CreateInTx(ctx, tx, p)
		})
	}

	// === Scenario A: tepat lunas (5 x 200rb = 1jt) ===
	invA, tglA := mkInvoice("A")
	pay200 := int64(20_000_000) // 200rb * 100
	var okA, failA atomic.Int32
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			if err := pay(invA, tglA, pay200); err != nil {
				failA.Add(1)
			} else {
				okA.Add(1)
			}
		}()
	}
	wg.Wait()
	var sumA int64
	_ = pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(jumlah),0) FROM pembayaran WHERE penjualan_id=$1 AND penjualan_tanggal=$2`,
		invA, tglA).Scan(&sumA)
	var statusA string
	_ = pool.QueryRow(context.Background(),
		`SELECT status_bayar FROM penjualan WHERE id=$1 AND tanggal=$2`,
		invA, tglA).Scan(&statusA)
	t.Logf("scenarioA: ok=%d fail=%d sum=%d status=%s", okA.Load(), failA.Load(), sumA, statusA)
	if sumA != totalCents {
		t.Errorf("scenarioA: expected sum=%d got %d", totalCents, sumA)
	}
	if statusA != "lunas" {
		t.Errorf("scenarioA: expected status_bayar=lunas got %s", statusA)
	}

	// === Scenario B: lebih (5 x 300rb = 1.5jt) — must reject excess ===
	invB, tglB := mkInvoice("B")
	pay300 := int64(30_000_000)
	var okB, failB atomic.Int32
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			if err := pay(invB, tglB, pay300); err != nil {
				failB.Add(1)
			} else {
				okB.Add(1)
			}
		}()
	}
	wg.Wait()
	var sumB int64
	_ = pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(jumlah),0) FROM pembayaran WHERE penjualan_id=$1 AND penjualan_tanggal=$2`,
		invB, tglB).Scan(&sumB)
	t.Logf("scenarioB: ok=%d fail=%d sum=%d (expected sum<=%d)", okB.Load(), failB.Load(), sumB, totalCents)
	if sumB > totalCents {
		t.Errorf("scenarioB: OVERPAYMENT! sum=%d > total=%d", sumB, totalCents)
	}
	// Maksimum 3 sukses (3*300rb=900rb<=1jt). Sukses ke-4 (1.2jt) harus ditolak.
	if okB.Load() > 3 {
		t.Errorf("scenarioB: expected ok<=3 got %d", okB.Load())
	}
}

// TestMutasiConcurrent — Mutasi A->B qty10 vs A->C qty10, stok asal=15.
// Salah satu HARUS gagal (insufficient). Stok akhir A = 5 (yg lewat) atau 15 (kalau dua2 gagal — tidak diharapkan).
// Tidak boleh negatif.
func TestMutasiConcurrent(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	f := newFixture(t, pool)

	// Replikasi flow Submit: lock stok asal FOR UPDATE, cek qty cukup, decrement.
	// Tidak pakai service penuh karena trigger DB sebenarnya melakukan decrement
	// pada update status. Untuk test isolation race detector kita simulasi
	// langsung step lock+update yg ekuivalen.
	seedStok(t, pool, f.gudangAID, f.produkID, 15)

	doMutasi := func(_ int64, qty float64) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			var current float64
			err := tx.QueryRow(ctx,
				`SELECT qty FROM stok WHERE gudang_id=$1 AND produk_id=$2 FOR UPDATE`,
				f.gudangAID, f.produkID).Scan(&current)
			if err != nil {
				return err
			}
			if current < qty {
				return fmt.Errorf("stok tidak cukup: %.4f < %.4f", current, qty)
			}
			// Sleep kecil supaya overlap critical section lebih obvious.
			time.Sleep(20 * time.Millisecond)
			_, err = tx.Exec(ctx,
				`UPDATE stok SET qty = qty - $3 WHERE gudang_id=$1 AND produk_id=$2`,
				f.gudangAID, f.produkID, qty)
			return err
		})
	}

	var ok, fail atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)
	for _, dest := range []int64{f.gudangBID, f.gudangCID} {
		dest := dest
		go func() {
			defer wg.Done()
			if err := doMutasi(dest, 10); err != nil {
				fail.Add(1)
			} else {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	final := getStok(t, pool, f.gudangAID, f.produkID)
	t.Logf("mutasi: ok=%d fail=%d final_stok_asal=%.4f", ok.Load(), fail.Load(), final)
	if final < 0 {
		t.Fatalf("STOK NEGATIF: %.4f", final)
	}
	if ok.Load() != 1 || fail.Load() != 1 {
		t.Errorf("expected exactly 1 ok + 1 fail, got ok=%d fail=%d", ok.Load(), fail.Load())
	}
	if final != 5 {
		t.Errorf("expected final stok=5 got %.4f", final)
	}
}

// Program migrate-excel: one-shot importer dari file Excel sistem lama
// (Mitra Usaha + Antar Gudang) ke PostgreSQL.
//
// Penggunaan:
//
//	migrate-excel \
//	  --source "/path/to/PROJECT UNTUK TOKOBANGUNAN" \
//	  --db-url "$DATABASE_URL" \
//	  --mode audit|dry-run|import \
//	  --year 2025 \
//	  --confirm-sayan SAYAN \
//	  --batch-size 1000 \
//	  --log-file migrate.log
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/omanjaya/tokobangunan/internal/importer/excel"
)

func main() {
	var (
		source        = flag.String("source", "", "path folder berisi file Excel (.xlsx)")
		dbURL         = flag.String("db-url", "", "Postgres connection string (default ke env DATABASE_URL)")
		mode          = flag.String("mode", "audit", "mode operasi: audit | dry-run | import")
		year          = flag.Int("year", 2025, "tahun data yang diimport (untuk verifikasi & partition)")
		confirmSayan  = flag.String("confirm-sayan", "SAYAN", "pilih file SAYAN: SAYAN atau SAYAN_1")
		batchSize     = flag.Int("batch-size", 1000, "ukuran batch insert untuk progress log")
		logFile       = flag.String("log-file", "", "tulis log ke file (selain stdout)")
		openingDate   = flag.String("opening-date", "2025-01-01", "tanggal opening untuk piutang awal (YYYY-MM-DD)")
	)
	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "error: --source wajib diisi")
		os.Exit(2)
	}

	_ = godotenv.Load()
	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}

	logger := buildLogger(*logFile)

	opening, err := time.Parse("2006-01-02", *openingDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening-date format invalid: %v\n", err)
		os.Exit(2)
	}

	opts := excel.ImportOptions{
		SourceDir:   *source,
		Mode:        excel.Mode(*mode),
		Year:        *year,
		SayanChoice: excel.SayanChoice(*confirmSayan),
		BatchSize:   *batchSize,
		OpeningDate: opening,
	}

	printHeader(opts, *dbURL)

	ctx := context.Background()

	// Mode audit tidak butuh DB.
	if opts.Mode == excel.ModeAudit {
		report, err := excel.AuditAll(*source)
		if err != nil {
			logger.Error("audit gagal", "err", err)
			os.Exit(1)
		}
		printAuditReport(report)
		return
	}

	if *dbURL == "" {
		fmt.Fprintln(os.Stderr, "error: --db-url atau env DATABASE_URL wajib untuk mode "+string(opts.Mode))
		os.Exit(2)
	}

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		logger.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("ping db", "err", err)
		os.Exit(1)
	}

	im := excel.NewImporter(pool, logger)
	summary, err := im.Run(ctx, opts)
	if err != nil {
		logger.Error("run gagal", "err", err)
		if summary != nil {
			printSummary(summary, opts)
		}
		os.Exit(1)
	}
	printSummary(summary, opts)
}

func buildLogger(logFile string) *slog.Logger {
	var w io.Writer = os.Stdout
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			w = io.MultiWriter(os.Stdout, f)
		}
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func printHeader(opts excel.ImportOptions, dbURL string) {
	fmt.Println("============================================================")
	fmt.Println("TOKOBANGUNAN -- MIGRASI EXCEL")
	fmt.Printf("Mode: %s\n", opts.Mode)
	fmt.Printf("Source: %s\n", opts.SourceDir)
	if opts.Mode != excel.ModeAudit {
		fmt.Printf("DB: %s\n", maskDBURL(dbURL))
	}
	fmt.Printf("Tahun: %d  Sayan: %s  Batch: %d\n", opts.Year, opts.SayanChoice, opts.BatchSize)
	fmt.Println("============================================================")
}

func maskDBURL(u string) string {
	if u == "" {
		return "(none)"
	}
	if len(u) > 60 {
		return u[:30] + "..." + u[len(u)-20:]
	}
	return u
}

func printAuditReport(r *excel.AuditReport) {
	fmt.Println()
	fmt.Println("[Audit Report]")
	fmt.Printf("Source: %s\n", r.SourceDir)
	fmt.Printf("Files: %d\n", len(r.Files))
	for _, f := range r.Files {
		fmt.Printf("  - %s (%.1f KB)  sheets=%d\n",
			f.Name, float64(f.Size)/1024.0, len(f.Sheets))
		for _, sh := range f.Sheets {
			if n := f.RowCount[sh]; n > 0 {
				fmt.Printf("      %-20s rows=%d\n", sh, n)
			}
		}
	}
	if r.SayanComparison != nil {
		c := r.SayanComparison
		fmt.Println()
		fmt.Println("[SAYAN comparison]")
		fmt.Printf("  hash equal: %v\n", c.IdenticalContent)
		fmt.Printf("  rows MAIN: %d vs %d\n", c.RowCountSheet1, c.RowCountSheet2)
		fmt.Printf("  rekomendasi: %s\n", c.Recommendation)
	}
	fmt.Println()
	fmt.Printf("[Master kandidat]\n")
	fmt.Printf("  Produk distinct: %d\n", len(r.ProdukCandidates))
	fmt.Printf("  Mitra distinct:  %d\n", len(r.MitraCandidates))
	if len(r.ProdukCandidates) > 0 {
		fmt.Println("  Top 10 produk:")
		limit := 10
		if len(r.ProdukCandidates) < limit {
			limit = len(r.ProdukCandidates)
		}
		for i := 0; i < limit; i++ {
			c := r.ProdukCandidates[i]
			fmt.Printf("    %4d  %s\n", c.Occurrence, c.NamaAsli)
		}
	}
	if len(r.MitraCandidates) > 0 {
		fmt.Println("  Top 10 mitra:")
		limit := 10
		if len(r.MitraCandidates) < limit {
			limit = len(r.MitraCandidates)
		}
		for i := 0; i < limit; i++ {
			c := r.MitraCandidates[i]
			fmt.Printf("    %4d  %s\n", c.Occurrence, c.NamaAsli)
		}
	}
	fmt.Println()
	fmt.Printf("Faktor konversi terdeteksi: %v\n", r.KonversiFactors)
	if len(r.Anomalies) > 0 {
		fmt.Printf("Anomali: %d\n", len(r.Anomalies))
		for i, a := range r.Anomalies {
			if i >= 20 {
				fmt.Printf("  ...dan %d lagi\n", len(r.Anomalies)-i)
				break
			}
			fmt.Printf("  - [%s/%s row %d] %s\n", a.File, a.Sheet, a.RowIdx, a.Reason)
		}
	}
}

func printSummary(s *excel.ImportSummary, opts excel.ImportOptions) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("VERIFIKASI")
	fmt.Println("============================================================")
	for _, v := range s.VerifikasiHasil {
		fmt.Printf("  %-12s %-7s  total DB Rp %d\n", v.GudangKode, v.Periode, v.TotalDB)
	}
	fmt.Println()
	fmt.Println("SUMMARY:")
	fmt.Printf("  Mode:               %s\n", opts.Mode)
	fmt.Printf("  Produk dibuat:      %d\n", s.TotalProdukDibuat)
	fmt.Printf("  Mitra dibuat:       %d\n", s.TotalMitraDibuat)
	fmt.Printf("  Penjualan import:   %d\n", s.PenjualanDiimport)
	fmt.Printf("  Mutasi import:      %d\n", s.MutasiDiimport)
	fmt.Printf("  Piutang opening:    %d\n", s.PiutangDiimport)
	fmt.Printf("  Pembayaran import:  %d\n", s.PembayaranDiimport)
	fmt.Printf("  Stok rows:          %d\n", s.StokRows)
	fmt.Printf("  Tabungan import:    %d\n", s.TabunganDiimport)
	fmt.Printf("  Errors:             %d\n", len(s.Errors))
	fmt.Printf("  Duration:           %s\n", s.Duration.Round(time.Millisecond))
	if len(s.Errors) > 0 {
		fmt.Println()
		fmt.Println("Top 20 errors:")
		for i, e := range s.Errors {
			if i >= 20 {
				fmt.Printf("  ...dan %d lagi\n", len(s.Errors)-i)
				break
			}
			fmt.Printf("  - [%s] %s/%s row %d: %s\n",
				e.Step, e.File, e.Sheet, e.RowIdx, e.Message)
		}
	}
	fmt.Println()
	fmt.Println("DONE.")
}

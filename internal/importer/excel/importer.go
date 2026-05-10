package excel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

const migrateUsername = "migrate"

// Mode operasi importer.
type Mode string

const (
	ModeAudit  Mode = "audit"
	ModeDryRun Mode = "dry-run"
	ModeImport Mode = "import"
)

// SayanChoice user pilih SAYAN atau SAYAN_1.
type SayanChoice string

const (
	SayanCurrent SayanChoice = "SAYAN"
	SayanAlt     SayanChoice = "SAYAN_1"
)

// ImportOptions parameter Run().
type ImportOptions struct {
	SourceDir    string
	Mode         Mode
	Year         int
	SayanChoice  SayanChoice
	BatchSize    int
	OpeningDate  time.Time // dipakai untuk piutang opening
}

// ImportError record kesalahan saat import.
type ImportError struct {
	Step    string
	File    string
	Sheet   string
	RowIdx  int
	Message string
}

// ImportSummary hasil akhir.
type ImportSummary struct {
	TotalProdukDibuat   int
	TotalMitraDibuat    int
	PenjualanDiimport   int
	MutasiDiimport      int
	PiutangDiimport     int
	PembayaranDiimport  int
	StokRows            int
	TabunganDiimport    int
	PembelianDiimport   int
	Errors              []ImportError
	Duration            time.Duration
	VerifikasiHasil     []VerifikasiResult
	AuditReport         *AuditReport
}

// Importer membungkus pool DB & logger.
type Importer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger

	// Run-scoped state (set di awal Run, dipakai per-step methods).
	opts      ImportOptions
	userID    int64
	produkMap map[string]int64
	mitraMap  map[string]int64
	gudangMap map[string]int64
	satuanMap map[string]int64
}

// NewImporter constructor.
func NewImporter(pool *pgxpool.Pool, logger *slog.Logger) *Importer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Importer{pool: pool, logger: logger}
}

// Run orkestrasi seluruh proses sesuai mode.
func (im *Importer) Run(ctx context.Context, opts ImportOptions) (*ImportSummary, error) {
	start := time.Now()
	summary := &ImportSummary{}

	im.logger.Info("audit excel files", "source", opts.SourceDir)
	report, err := AuditAll(opts.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	summary.AuditReport = report
	im.logger.Info("audit selesai",
		"files", len(report.Files),
		"produk_kandidat", len(report.ProdukCandidates),
		"mitra_kandidat", len(report.MitraCandidates))

	if opts.Mode == ModeAudit {
		summary.Duration = time.Since(start)
		return summary, nil
	}

	// Resolve user 'migrate'.
	userID, err := im.ensureMigrateUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("ensure migrate user: %w", err)
	}

	// Build master data & import transaksi.
	produkMap, err := im.BuildMasterProduk(ctx, report.ProdukCandidates)
	if err != nil {
		return nil, fmt.Errorf("master produk: %w", err)
	}
	summary.TotalProdukDibuat = len(produkMap)
	im.logger.Info("master produk siap", "total", len(produkMap))

	mitraMap, err := im.BuildMasterMitra(ctx, report.MitraCandidates)
	if err != nil {
		return nil, fmt.Errorf("master mitra: %w", err)
	}
	summary.TotalMitraDibuat = len(mitraMap)
	im.logger.Info("master mitra siap", "total", len(mitraMap))

	gudangMap, err := im.loadGudangMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("load gudang: %w", err)
	}
	satuanMap, err := im.loadSatuanMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("load satuan: %w", err)
	}

	// Simpan state run-scoped agar per-step methods bisa pakai.
	im.opts = opts
	im.userID = userID
	im.produkMap = produkMap
	im.mitraMap = mitraMap
	im.gudangMap = gudangMap
	im.satuanMap = satuanMap

	// Pilih file mitra yang dipakai (skip SAYAN(1) sesuai pilihan).
	for _, f := range report.Files {
		if err := im.processMitraFile(ctx, f, summary); err != nil {
			return summary, err
		}
	}

	if err := im.processAntarGudang(ctx, report, summary); err != nil {
		return summary, err
	}

	// Verifikasi.
	if opts.Mode == ModeImport {
		ver, err := im.VerifyMigration(ctx, opts.Year)
		if err != nil {
			im.logger.Warn("verifikasi gagal", "err", err)
		}
		summary.VerifikasiHasil = ver
	}

	summary.Duration = time.Since(start)
	return summary, nil
}

// processMitraFile menangani satu file mitra: open workbook lalu jalankan semua step.
func (im *Importer) processMitraFile(ctx context.Context, f FileInfo, summary *ImportSummary) error {
	gudangID, gudangKode, ok := im.resolveGudangFromFile(f, summary)
	if !ok {
		return nil
	}
	wb, err := OpenWorkbook(f.Path)
	if err != nil {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "open", File: f.Name, Message: err.Error(),
		})
		return nil
	}
	defer wb.Close()

	steps := []func() error{
		func() error { return im.processMainPenjualan(ctx, wb, gudangID, gudangKode, f.Name, summary) },
		func() error { return im.processPembayaran(ctx, wb, gudangKode, summary) },
		func() error { return im.processPiutangOpening(ctx, wb, gudangID, gudangKode, summary) },
		func() error { return im.processStokGudang(ctx, wb, gudangID, gudangKode, summary) },
		func() error { return im.processTabungan(ctx, wb, gudangKode, f.Name, summary) },
		func() error { return im.processHutang(ctx, wb, gudangID, gudangKode, f.Name, summary) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// resolveGudangFromFile filter file mitra & resolve gudang ID. Return ok=false untuk skip.
func (im *Importer) resolveGudangFromFile(f FileInfo, summary *ImportSummary) (int64, string, bool) {
	up := strings.ToUpper(f.Name)
	if !strings.Contains(up, "MITRA USAHA") {
		return 0, "", false
	}
	isSayanAlt := strings.Contains(up, "SAYAN") && strings.Contains(up, "(1)")
	isSayanMain := strings.Contains(up, "SAYAN") && !strings.Contains(up, "(1)")
	if im.opts.SayanChoice == SayanCurrent && isSayanAlt {
		im.logger.Info("skip file (SAYAN choice)", "file", f.Name)
		return 0, "", false
	}
	if im.opts.SayanChoice == SayanAlt && isSayanMain {
		im.logger.Info("skip file (SAYAN choice)", "file", f.Name)
		return 0, "", false
	}
	gudangKode := detectCabang(f.Name)
	gudangID, ok := im.gudangMap[gudangKode]
	if !ok {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "resolve_gudang", File: f.Name,
			Message: fmt.Sprintf("gudang %s tidak ditemukan di DB", gudangKode),
		})
		return 0, "", false
	}
	return gudangID, gudangKode, true
}

// processMainPenjualan parse MAIN sheet & import penjualan.
func (im *Importer) processMainPenjualan(ctx context.Context, wb *Workbook, gudangID int64, gudangKode, fileName string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "MAIN") {
		return nil
	}
	pjr, anoms, err := ParseMitraMain(wb, "MAIN", gudangKode)
	if err != nil {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_main", File: fileName, Message: err.Error(),
		})
	}
	for _, a := range anoms {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_main", File: a.File, Sheet: a.Sheet, RowIdx: a.RowIdx, Message: a.Reason,
		})
	}
	im.logger.Info("parse MAIN selesai", "gudang", gudangKode, "rows_terbaca", len(pjr))
	n, err := im.ImportPenjualan(ctx, im.opts, gudangID, im.userID, pjr, im.produkMap, im.mitraMap, im.satuanMap)
	if err != nil {
		return fmt.Errorf("import penjualan %s: %w", gudangKode, err)
	}
	summary.PenjualanDiimport += n
	im.logger.Info("import penjualan selesai", "gudang", gudangKode, "rows", n)
	return nil
}

// processPembayaran parse Pembayaran sheet & import.
func (im *Importer) processPembayaran(ctx context.Context, wb *Workbook, gudangKode string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "Pembayaran") {
		return nil
	}
	pbr, anoms, err := ParsePembayaran(wb, "Pembayaran", gudangKode)
	if err == nil {
		n, err := im.ImportPembayaran(ctx, im.opts, im.userID, pbr, im.mitraMap)
		if err != nil {
			return fmt.Errorf("import pembayaran %s: %w", gudangKode, err)
		}
		summary.PembayaranDiimport += n
		im.logger.Info("import pembayaran selesai", "gudang", gudangKode, "rows", n)
	}
	for _, a := range anoms {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_pembayaran", File: a.File, Sheet: a.Sheet, RowIdx: a.RowIdx, Message: a.Reason,
		})
	}
	return nil
}

// processPiutangOpening parse PIUTANG sheet & import opening balance.
func (im *Importer) processPiutangOpening(ctx context.Context, wb *Workbook, gudangID int64, gudangKode string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "PIUTANG") {
		return nil
	}
	pt, _ := ParsePiutang(wb, "PIUTANG", gudangKode, im.opts.OpeningDate)
	n, err := im.ImportPiutangOpening(ctx, im.opts, gudangID, im.userID, pt, im.mitraMap)
	if err != nil {
		return fmt.Errorf("import piutang %s: %w", gudangKode, err)
	}
	summary.PiutangDiimport += n
	im.logger.Info("import piutang opening selesai", "gudang", gudangKode, "rows", n)
	return nil
}

// processStokGudang parse Stok Gudang sheet & import.
func (im *Importer) processStokGudang(ctx context.Context, wb *Workbook, gudangID int64, gudangKode string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "Stok Gudang") {
		return nil
	}
	st, _ := ParseStokGudang(wb, "Stok Gudang", gudangKode)
	n, err := im.ImportStok(ctx, im.opts, gudangID, st, im.produkMap)
	if err != nil {
		return fmt.Errorf("import stok %s: %w", gudangKode, err)
	}
	summary.StokRows += n
	return nil
}

// processTabungan parse Tabungan sheet & import.
func (im *Importer) processTabungan(ctx context.Context, wb *Workbook, gudangKode, fileName string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "Tabungan") {
		return nil
	}
	tb, anoms, err := ParseTabungan(wb, "Tabungan", gudangKode)
	if err != nil {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_tabungan", File: fileName, Message: err.Error(),
		})
	}
	for _, a := range anoms {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_tabungan", File: a.File, Sheet: a.Sheet, RowIdx: a.RowIdx, Message: a.Reason,
		})
	}
	n, err := im.ImportTabungan(ctx, im.opts, im.userID, tb, im.mitraMap)
	if err != nil {
		return fmt.Errorf("import tabungan %s: %w", gudangKode, err)
	}
	summary.TabunganDiimport += n
	im.logger.Info("import tabungan selesai", "gudang", gudangKode, "rows", n)
	return nil
}

// processHutang parse Hutang sheet & import pembelian.
func (im *Importer) processHutang(ctx context.Context, wb *Workbook, gudangID int64, gudangKode, fileName string, summary *ImportSummary) error {
	if !hasSheet(wb.Sheets(), "Hutang") {
		return nil
	}
	pb, anoms, err := ParseHutang(wb, "Hutang", gudangKode)
	if err != nil {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_hutang", File: fileName, Message: err.Error(),
		})
	}
	for _, a := range anoms {
		summary.Errors = append(summary.Errors, ImportError{
			Step: "parse_hutang", File: a.File, Sheet: a.Sheet, RowIdx: a.RowIdx, Message: a.Reason,
		})
	}
	n, err := im.ImportPembelian(ctx, im.opts, gudangID, im.userID, pb, im.produkMap, im.satuanMap)
	if err != nil {
		return fmt.Errorf("import pembelian %s: %w", gudangKode, err)
	}
	summary.PembelianDiimport += n
	im.logger.Info("import pembelian selesai", "gudang", gudangKode, "rows", n)
	return nil
}

// processAntarGudang parse file Antar Gudang & import mutasi.
func (im *Importer) processAntarGudang(ctx context.Context, report *AuditReport, summary *ImportSummary) error {
	for _, f := range report.Files {
		if !strings.EqualFold(f.Name, "Antar Gudang 2025.xlsx") {
			continue
		}
		wb, err := OpenWorkbook(f.Path)
		if err != nil {
			summary.Errors = append(summary.Errors, ImportError{
				Step: "open_antar_gudang", File: f.Name, Message: err.Error(),
			})
			continue
		}
		mu, _, _ := ParseAntarGudang(wb)
		n, err := im.ImportMutasi(ctx, im.opts, im.userID, mu, im.produkMap, im.gudangMap, im.satuanMap)
		if err != nil {
			_ = wb.Close()
			return fmt.Errorf("import mutasi: %w", err)
		}
		summary.MutasiDiimport += n
		_ = wb.Close()
		im.logger.Info("import mutasi selesai", "rows", n)
	}
	return nil
}

// ensureMigrateUser create user 'migrate' kalau belum ada.
func (im *Importer) ensureMigrateUser(ctx context.Context) (int64, error) {
	var id int64
	err := im.pool.QueryRow(ctx,
		`SELECT id FROM "user" WHERE username = $1`, migrateUsername).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}
	pw, err := auth.GenerateRandomPassword(20)
	if err != nil {
		return 0, err
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return 0, err
	}
	err = im.pool.QueryRow(ctx, `
		INSERT INTO "user" (username, password_hash, nama_lengkap, role, gudang_id, is_active)
		VALUES ($1, $2, 'Migrasi Excel', 'admin', NULL, FALSE)
		RETURNING id
	`, migrateUsername, hash).Scan(&id)
	if err != nil {
		return 0, err
	}
	im.logger.Info("created migrate user", "id", id, "username", migrateUsername)
	return id, nil
}

func (im *Importer) loadGudangMap(ctx context.Context) (map[string]int64, error) {
	rows, err := im.pool.Query(ctx, `SELECT id, kode FROM gudang`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var kode string
		if err := rows.Scan(&id, &kode); err != nil {
			return nil, err
		}
		out[strings.ToUpper(kode)] = id
	}
	return out, rows.Err()
}

func (im *Importer) loadSatuanMap(ctx context.Context) (map[string]int64, error) {
	rows, err := im.pool.Query(ctx, `SELECT id, kode FROM satuan`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var kode string
		if err := rows.Scan(&id, &kode); err != nil {
			return nil, err
		}
		out[strings.ToLower(kode)] = id
	}
	return out, rows.Err()
}

// BuildMasterProduk insert produk distinct (idempotent by nama).
// Return mapping nama_normalisasi -> produk_id.
func (im *Importer) BuildMasterProduk(ctx context.Context, candidates []ProdukCandidate) (map[string]int64, error) {
	out := map[string]int64{}
	satuanMap, err := im.loadSatuanMap(ctx)
	if err != nil {
		return nil, err
	}
	defaultSatuan, ok := satuanMap["sak"]
	if !ok {
		defaultSatuan, ok = satuanMap["biji"]
		if !ok {
			return nil, fmt.Errorf("satuan default (sak/biji) tidak ada di DB")
		}
	}

	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Load existing.
	rows, err := tx.Query(ctx, `SELECT id, UPPER(nama) FROM produk WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err != nil {
			rows.Close()
			return nil, err
		}
		out[NormalizeProdukName(nama)] = id
	}
	rows.Close()

	seq := len(out) + 1
	for _, c := range candidates {
		if _, ok := out[c.Nama]; ok {
			continue
		}
		// Pilih satuan dari occurrence terbanyak kalau cocok dengan satuan_map.
		satuanID := defaultSatuan
		bestCount := 0
		for s, cnt := range c.Satuan {
			if id, ok := satuanMap[strings.ToLower(s)]; ok && cnt > bestCount {
				satuanID = id
				bestCount = cnt
			}
		}
		sku := fmt.Sprintf("P-%05d", seq)
		seq++
		var newID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO produk (sku, nama, satuan_kecil_id, faktor_konversi, is_active)
			VALUES ($1, $2, $3, 1, TRUE)
			RETURNING id
		`, sku, c.NamaAsli, satuanID).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("insert produk %s: %w", c.NamaAsli, err)
		}
		out[c.Nama] = newID
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// BuildMasterMitra insert mitra distinct.
func (im *Importer) BuildMasterMitra(ctx context.Context, candidates []MitraCandidate) (map[string]int64, error) {
	out := map[string]int64{}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `SELECT id, UPPER(nama) FROM mitra WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err != nil {
			rows.Close()
			return nil, err
		}
		out[NormalizeMitraName(nama)] = id
	}
	rows.Close()

	seq := len(out) + 1
	for _, c := range candidates {
		if _, ok := out[c.Nama]; ok {
			continue
		}
		kode := fmt.Sprintf("M-%05d", seq)
		seq++
		var id int64
		err := tx.QueryRow(ctx, `
			INSERT INTO mitra (kode, nama, tipe, limit_kredit, jatuh_tempo_hari)
			VALUES ($1, $2, 'eceran', 0, 30)
			RETURNING id
		`, kode, c.NamaAsli).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert mitra %s: %w", c.NamaAsli, err)
		}
		out[c.Nama] = id
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// ImportPenjualan batch insert penjualan + item. Idempotent via client_uuid.
func (im *Importer) ImportPenjualan(
	ctx context.Context, opts ImportOptions, gudangID, userID int64,
	rows []PenjualanRow,
	produkMap, mitraMap, satuanMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	defaultSatuan := firstSatuan(satuanMap)
	imported := 0
	for i, r := range rows {
		mitraID, ok := mitraMap[NormalizeMitraName(r.MitraNama)]
		if !ok {
			continue
		}
		produkID, ok := produkMap[NormalizeProdukName(r.ProdukNama)]
		if !ok {
			continue
		}
		clientUUID := deterministicUUID(r.SourceFile, r.SourceSh, r.RowIdx)
		nomor := fmt.Sprintf("MIG-%s-%d", r.GudangKode, r.RowIdx)
		statusBayar := "lunas"

		// Get satuan: ambil produk satuan_kecil_id.
		var satuanID int64
		var satuanKode string
		err := tx.QueryRow(ctx, `
			SELECT s.id, s.kode FROM produk p
			JOIN satuan s ON s.id = p.satuan_kecil_id
			WHERE p.id = $1
		`, produkID).Scan(&satuanID, &satuanKode)
		if err != nil {
			satuanID = defaultSatuan
			satuanKode = ""
		}
		if r.Satuan != "" {
			if id, ok := satuanMap[strings.ToLower(r.Satuan)]; ok {
				satuanID = id
				satuanKode = strings.ToLower(r.Satuan)
			}
		}

		// Convert rupiah utuh → cents (×100) untuk schema BIGINT cents.
		totalCents := r.Total * 100
		hargaCents := r.Harga * 100

		var pjID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO penjualan
				(nomor_kwitansi, tanggal, mitra_id, gudang_id, user_id,
				 subtotal, diskon, total, status_bayar, client_uuid)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $6, $7, $8)
			ON CONFLICT (client_uuid, tanggal) DO NOTHING
			RETURNING id
		`, nomor, r.Tanggal, mitraID, gudangID, userID, totalCents, statusBayar, clientUUID).Scan(&pjID)
		if err == pgx.ErrNoRows {
			// Sudah ada → skip.
			continue
		}
		if err != nil {
			return imported, fmt.Errorf("insert penjualan row %d: %w", r.RowIdx, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO penjualan_item
				(penjualan_id, penjualan_tanggal, produk_id, produk_nama,
				 qty, satuan_id, satuan_kode, qty_konversi, harga_satuan, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $5, $8, $9)
		`, pjID, r.Tanggal, produkID, r.ProdukNama, r.Qty, satuanID, satuanKode, hargaCents, totalCents)
		if err != nil {
			return imported, fmt.Errorf("insert penjualan_item row %d: %w", r.RowIdx, err)
		}
		imported++

		if (i+1)%opts.BatchSize == 0 {
			im.logger.Info("progress penjualan",
				"gudang", r.GudangKode, "done", i+1, "total", len(rows))
		}
	}

	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return imported, err
	}
	return imported, nil
}

// ImportMutasi insert mutasi_gudang + mutasi_item. Status diset 'draft'
// kemudian update ke 'diterima' supaya trigger stok jalan.
func (im *Importer) ImportMutasi(
	ctx context.Context, opts ImportOptions, userID int64,
	rows []MutasiRow,
	produkMap, gudangMap, satuanMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	imported := 0
	for _, r := range rows {
		asalID, ok := gudangMap[r.GudangAsal]
		if !ok {
			continue
		}
		tujuanID, ok := gudangMap[r.GudangTujuan]
		if !ok {
			continue
		}
		key := NormalizeProdukName(r.ProdukNama)
		produkID, ok := produkMap[key]
		if !ok {
			// Buat produk on-the-fly kalau ditemukan di mutasi tapi belum di master.
			satID := firstSatuan(satuanMap)
			sku := fmt.Sprintf("MIG-%04d", len(produkMap)+1)
			err := tx.QueryRow(ctx, `
				INSERT INTO produk (sku, nama, satuan_kecil_id, faktor_konversi, is_active)
				VALUES ($1, $2, $3, 1, true)
				ON CONFLICT (sku) DO UPDATE SET sku = EXCLUDED.sku
				RETURNING id
			`, sku, r.ProdukNama, satID).Scan(&produkID)
			if err != nil {
				continue
			}
			produkMap[key] = produkID
		}
		clientUUID := deterministicUUID(r.SourceFile, r.SourceSh, r.RowIdx)
		nomor := fmt.Sprintf("MUT-MIG-%s-%s-%d", r.GudangAsal, r.GudangTujuan, r.RowIdx)

		var satuanID int64
		var satuanKode string
		err := tx.QueryRow(ctx, `
			SELECT s.id, s.kode FROM produk p
			JOIN satuan s ON s.id = p.satuan_kecil_id WHERE p.id = $1
		`, produkID).Scan(&satuanID, &satuanKode)
		if err != nil {
			satuanID = firstSatuan(satuanMap)
		}

		var mutID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO mutasi_gudang
				(nomor_mutasi, tanggal, gudang_asal_id, gudang_tujuan_id, status,
				 user_pengirim_id, user_penerima_id, tanggal_kirim, tanggal_terima,
				 client_uuid)
			VALUES ($1, $2::date, $3, $4, 'draft', $5, $5, $2::date::timestamptz, $2::date::timestamptz, $6)
			ON CONFLICT (client_uuid) DO NOTHING
			RETURNING id
		`, nomor, r.Tanggal, asalID, tujuanID, userID, clientUUID).Scan(&mutID)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return imported, fmt.Errorf("insert mutasi: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO mutasi_item
				(mutasi_id, produk_id, produk_nama, qty, satuan_id, satuan_kode, qty_konversi, harga_internal)
			VALUES ($1, $2, $3, $4, $5, $6, $4, $7)
		`, mutID, produkID, r.ProdukNama, r.Qty, satuanID, satuanKode, r.HargaInternal)
		if err != nil {
			return imported, err
		}

		// Progress: draft -> dikirim -> diterima (trigger update stok).
		if _, err := tx.Exec(ctx,
			`UPDATE mutasi_gudang SET status='dikirim' WHERE id=$1`, mutID); err != nil {
			return imported, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE mutasi_gudang SET status='diterima' WHERE id=$1`, mutID); err != nil {
			return imported, err
		}
		imported++
	}

	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

// ImportPembayaran insert pembayaran (tidak link ke penjualan tertentu).
func (im *Importer) ImportPembayaran(
	ctx context.Context, opts ImportOptions, userID int64,
	rows []PembayaranRow, mitraMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	imported := 0
	for i, r := range rows {
		mitraID, ok := mitraMap[NormalizeMitraName(r.MitraNama)]
		if !ok {
			continue
		}
		clientUUID := deterministicUUID("PEMBAYARAN-"+r.GudangKode, r.MitraNama, i)
		jumlahCents := r.Jumlah * 100
		_, err := tx.Exec(ctx, `
			INSERT INTO pembayaran (mitra_id, tanggal, jumlah, metode, referensi, user_id, client_uuid)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (client_uuid) DO NOTHING
		`, mitraID, r.Tanggal, jumlahCents, defaultStr(r.Metode, "tunai"), nilable(r.Referensi), userID, clientUUID)
		if err != nil {
			return imported, fmt.Errorf("insert pembayaran: %w", err)
		}
		imported++
	}
	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

// ImportPiutangOpening generate phantom penjualan kredit.
func (im *Importer) ImportPiutangOpening(
	ctx context.Context, opts ImportOptions, gudangID, userID int64,
	rows []PiutangAwal, mitraMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	imported := 0
	for i, r := range rows {
		mitraID, ok := mitraMap[NormalizeMitraName(r.MitraNama)]
		if !ok {
			continue
		}
		clientUUID := deterministicUUID("PIUTANG-OPEN-"+r.GudangKode, r.MitraNama, i)
		nomor := fmt.Sprintf("OPEN-%s-%05d", r.GudangKode, i+1)
		saldoCents := r.Saldo * 100
		_, err := tx.Exec(ctx, `
			INSERT INTO penjualan
				(nomor_kwitansi, tanggal, mitra_id, gudang_id, user_id,
				 subtotal, diskon, total, status_bayar, client_uuid, catatan)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $6, 'kredit', $7, 'Saldo piutang awal (migrasi Excel)')
			ON CONFLICT (client_uuid, tanggal) DO NOTHING
		`, nomor, r.Tanggal, mitraID, gudangID, userID, saldoCents, clientUUID)
		if err != nil {
			return imported, fmt.Errorf("insert piutang opening: %w", err)
		}
		imported++
	}
	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

// ImportStok upsert stok awal.
func (im *Importer) ImportStok(
	ctx context.Context, opts ImportOptions, gudangID int64,
	rows []StokRow, produkMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	imported := 0
	for _, r := range rows {
		produkID, ok := produkMap[NormalizeProdukName(r.ProdukNama)]
		if !ok {
			continue
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO stok (gudang_id, produk_id, qty)
			VALUES ($1, $2, $3)
			ON CONFLICT (gudang_id, produk_id) DO UPDATE SET qty = EXCLUDED.qty, updated_at = now()
		`, gudangID, produkID, r.Qty)
		if err != nil {
			return imported, err
		}
		imported++
	}
	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

// ensureSupplier resolve supplier id by nama (case-insensitive). Buat baru
// kalau belum ada. Supplier bersifat global (tidak per-gudang).
func (im *Importer) ensureSupplier(ctx context.Context, tx pgx.Tx, nama string, cache map[string]int64) (int64, error) {
	key := strings.ToUpper(strings.TrimSpace(nama))
	if id, ok := cache[key]; ok {
		return id, nil
	}
	var id int64
	err := tx.QueryRow(ctx,
		`SELECT id FROM supplier WHERE UPPER(nama) = $1 AND deleted_at IS NULL LIMIT 1`,
		key).Scan(&id)
	if err == nil {
		cache[key] = id
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}
	// Buat baru. Kode synthesized.
	kode := fmt.Sprintf("S-%05d", len(cache)+1)
	// Pastikan kode unique (loop kalau bentrok).
	for i := 0; i < 5; i++ {
		err = tx.QueryRow(ctx, `
			INSERT INTO supplier (kode, nama, is_active)
			VALUES ($1, $2, TRUE)
			ON CONFLICT (kode) DO NOTHING
			RETURNING id
		`, kode, nama).Scan(&id)
		if err == nil {
			cache[key] = id
			return id, nil
		}
		if err != pgx.ErrNoRows {
			return 0, fmt.Errorf("insert supplier %s: %w", nama, err)
		}
		kode = fmt.Sprintf("S-%05d-%d", len(cache)+1, i+1)
	}
	return 0, fmt.Errorf("ensureSupplier %s: gagal generate kode unik", nama)
}

// ImportPembelian insert pembelian header + item dari sheet Hutang.
// 1 baris = 1 pembelian (sheet Excel sudah granular per item). Idempotency
// via nomor_pembelian deterministic (UNIQUE).
func (im *Importer) ImportPembelian(
	ctx context.Context, opts ImportOptions, gudangID, userID int64,
	rows []PembelianRow,
	produkMap, satuanMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	supplierCache := map[string]int64{}
	defaultSatuan := firstSatuan(satuanMap)
	imported := 0
	for i, r := range rows {
		produkID, ok := produkMap[NormalizeProdukName(r.ProdukNama)]
		if !ok {
			im.logger.Warn("pembelian: produk tidak dikenal",
				"gudang", r.GudangKode, "produk", r.ProdukNama, "row", i+1)
			continue
		}
		supplierID, err := im.ensureSupplier(ctx, tx, r.Supplier, supplierCache)
		if err != nil {
			return imported, err
		}

		// Resolve satuan: prefer dari row, fallback satuan_kecil_id produk.
		var satuanID int64
		var satuanKode string
		if r.Satuan != "" {
			if id, ok := satuanMap[strings.ToLower(r.Satuan)]; ok {
				satuanID = id
				satuanKode = strings.ToLower(r.Satuan)
			}
		}
		if satuanID == 0 {
			err := tx.QueryRow(ctx, `
				SELECT s.id, s.kode FROM produk p
				JOIN satuan s ON s.id = p.satuan_kecil_id
				WHERE p.id = $1
			`, produkID).Scan(&satuanID, &satuanKode)
			if err != nil {
				satuanID = defaultSatuan
				satuanKode = ""
			}
		}

		totalCents := r.Total * 100
		hargaCents := r.Harga * 100

		// Deterministic nomor: hash file/sheet/row sebagai natural key.
		hash := deterministicUUID("PEMBELIAN", r.GudangKode, r.Tanggal.Format("2006-01-02"), r.Supplier, r.ProdukNama, i)
		nomor := fmt.Sprintf("BUY-MIG-%s-%s", r.GudangKode, hash.String()[:8])

		var pbID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO pembelian
				(nomor_pembelian, tanggal, supplier_id, gudang_id, user_id,
				 subtotal, diskon, total, status_bayar, catatan)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $6, 'kredit', 'Migrasi Excel: Hutang')
			ON CONFLICT (nomor_pembelian) DO NOTHING
			RETURNING id
		`, nomor, r.Tanggal, supplierID, gudangID, userID, totalCents).Scan(&pbID)
		if err == pgx.ErrNoRows {
			// Sudah ada → skip (juga skip insert item agar trigger stok tak double).
			continue
		}
		if err != nil {
			return imported, fmt.Errorf("insert pembelian row %d: %w", i+1, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO pembelian_item
				(pembelian_id, produk_id, produk_nama, qty, satuan_id, satuan_kode,
				 qty_konversi, harga_satuan, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $4, $7, $8)
		`, pbID, produkID, r.ProdukNama, r.Qty, satuanID, satuanKode, hargaCents, totalCents)
		if err != nil {
			return imported, fmt.Errorf("insert pembelian_item row %d: %w", i+1, err)
		}
		imported++

		if opts.BatchSize > 0 && (i+1)%opts.BatchSize == 0 {
			im.logger.Info("progress pembelian",
				"gudang", r.GudangKode, "done", i+1, "total", len(rows))
		}
	}

	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

// ImportTabungan insert mutasi tabungan + hitung saldo running.
func (im *Importer) ImportTabungan(
	ctx context.Context, opts ImportOptions, userID int64,
	rows []TabunganRow, mitraMap map[string]int64,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	saldoCache := map[int64]int64{}
	imported := 0
	for _, r := range rows {
		mitraID, ok := mitraMap[NormalizeMitraName(r.MitraNama)]
		if !ok {
			continue
		}
		// Load saldo terakhir kalau belum cached.
		if _, exists := saldoCache[mitraID]; !exists {
			var s int64
			err := tx.QueryRow(ctx,
				`SELECT COALESCE(saldo, 0) FROM tabungan_mitra
				   WHERE mitra_id = $1 ORDER BY id DESC LIMIT 1`,
				mitraID).Scan(&s)
			if err != nil && err != pgx.ErrNoRows {
				return imported, err
			}
			saldoCache[mitraID] = s
		}
		saldoCache[mitraID] += r.Debit - r.Kredit
		_, err := tx.Exec(ctx, `
			INSERT INTO tabungan_mitra (mitra_id, tanggal, debit, kredit, saldo, catatan, user_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, mitraID, r.Tanggal, r.Debit, r.Kredit, saldoCache[mitraID], nilable(r.Catatan), userID)
		if err != nil {
			return imported, err
		}
		imported++
	}
	if opts.Mode == ModeDryRun {
		_ = tx.Rollback(ctx)
		return imported, nil
	}
	return imported, tx.Commit(ctx)
}

func deterministicUUID(parts ...interface{}) uuid.UUID {
	s := fmt.Sprint(parts...)
	h := sha256.Sum256([]byte(s))
	// Format ke UUID v5-like (deterministic, bukan random).
	hex.EncodeToString(h[:])
	var u uuid.UUID
	copy(u[:], h[:16])
	// Set version 5 + variant bits.
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

func firstSatuan(m map[string]int64) int64 {
	for _, id := range m {
		return id
	}
	return 0
}

func defaultStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func nilable(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// FilenameFromPath helper.
func FilenameFromPath(p string) string { return filepath.Base(p) }

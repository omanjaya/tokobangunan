package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/errgroup"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/laporan"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// LaporanHandler - HTTP handler untuk modul laporan.
type LaporanHandler struct {
	laporan  *service.LaporanService
	gudang   *service.GudangService
	cashflow *service.CashflowService // optional; nil-safe
}

func NewLaporanHandler(ls *service.LaporanService, gs *service.GudangService) *LaporanHandler {
	return &LaporanHandler{laporan: ls, gudang: gs}
}

// SetCashflow attach cashflow service (untuk laporan/cashflow).
// Dipisah supaya constructor lama tidak break.
func (h *LaporanHandler) SetCashflow(cs *service.CashflowService) {
	h.cashflow = cs
}

// Index GET /laporan - landing page (grid card).
func (h *LaporanHandler) Index(c echo.Context) error {
	props := laporan.IndexProps{
		Nav:  layout.DefaultNav("/laporan"),
		User: userData(auth.CurrentUser(c)),
	}
	return RenderHTML(c, http.StatusOK, laporan.Index(props))
}

// LR GET /laporan/lr.
func (h *LaporanHandler) LR(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.laporan.LR(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	props := laporan.LRProps{
		Nav:  layout.DefaultNav("/laporan"),
		User: userData(auth.CurrentUser(c)),
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Rows: rows,
	}
	return RenderHTML(c, http.StatusOK, laporan.LR(props))
}

// Penjualan GET /laporan/penjualan.
func (h *LaporanHandler) Penjualan(c echo.Context) error {
	from, to := parseRange(c)

	var gudangID, mitraID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}
	if m, _ := strconv.ParseInt(c.QueryParam("mitra_id"), 10, 64); m > 0 {
		mitraID = &m
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	filter := service.LaporanPenjualanFilter{
		From: from, To: to, GudangID: gudangID, MitraID: mitraID,
		Page: page, PerPage: 50,
	}
	// Parallelize report query + gudang dropdown lookup.
	var (
		rows    []repo.LaporanPenjualanRow
		total   int
		gudangs []laporan.GudangLite
	)
	g, gctx := errgroup.WithContext(c.Request().Context())
	g.Go(func() error {
		var e error
		rows, total, e = h.laporan.Penjualan(gctx, filter)
		return e
	})
	g.Go(func() error {
		var e error
		gudangs, e = h.gudangLiteCtx(gctx)
		return e
	})
	if err := g.Wait(); err != nil {
		return err
	}

	totalPages := total / filter.PerPage
	if total%filter.PerPage != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	gid := int64(0)
	if gudangID != nil {
		gid = *gudangID
	}
	mid := int64(0)
	if mitraID != nil {
		mid = *mitraID
	}

	props := laporan.PenjualanProps{
		Nav:        layout.DefaultNav("/laporan"),
		User:       userData(auth.CurrentUser(c)),
		From:       from.Format("2006-01-02"),
		To:         to.Format("2006-01-02"),
		Rows:       rows,
		Total:      total,
		Page:       page,
		PerPage:    filter.PerPage,
		TotalPages: totalPages,
		GudangID:   gid,
		MitraID:    mid,
		Gudangs:    gudangs,
	}
	return RenderHTML(c, http.StatusOK, laporan.Penjualan(props))
}

// ExportPenjualan GET /laporan/penjualan/export.csv.
func (h *LaporanHandler) ExportPenjualan(c echo.Context) error {
	from, to := parseRange(c)

	var gudangID, mitraID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}
	if m, _ := strconv.ParseInt(c.QueryParam("mitra_id"), 10, 64); m > 0 {
		mitraID = &m
	}

	filter := service.LaporanPenjualanFilter{
		From: from, To: to, GudangID: gudangID, MitraID: mitraID,
		Page: 1, PerPage: 200,
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="laporan-penjualan-%s_%s.csv"`,
			from.Format("20060102"), to.Format("20060102")))
	c.Response().WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Response().Writer)
	_ = w.Write([]string{"Tanggal", "Nomor", "Gudang", "Mitra", "Total", "Status"})

	for {
		rows, total, err := h.laporan.Penjualan(c.Request().Context(), filter)
		if err != nil {
			return err
		}
		for _, r := range rows {
			_ = w.Write([]string{
				r.Tanggal.Format("2006-01-02"),
				r.NomorKwitansi,
				r.GudangNama,
				r.MitraNama,
				strconv.FormatInt(r.Total/100, 10),
				r.StatusBayar,
			})
		}
		w.Flush()
		if filter.Page*filter.PerPage >= total || len(rows) == 0 {
			break
		}
		filter.Page++
	}
	return nil
}

// ExportPenjualanXLSX GET /laporan/penjualan/export.xlsx.
func (h *LaporanHandler) ExportPenjualanXLSX(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.collectPenjualanAll(c, from, to)
	if err != nil {
		return err
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Penjualan"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	headers := []string{"Tanggal", "Nomor Kwitansi", "Gudang", "Mitra", "Total", "Status"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E5E7EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	first, _ := excelize.CoordinatesToCellName(1, 1)
	last, _ := excelize.CoordinatesToCellName(len(headers), 1)
	_ = f.SetCellStyle(sheet, first, last, headerStyle)

	for r, row := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r+2), row.Tanggal.Format("2006-01-02"))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r+2), row.NomorKwitansi)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r+2), row.GudangNama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", r+2), row.MitraNama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", r+2), row.Total/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", r+2), row.StatusBayar)
	}

	// Auto-fit kolom (estimasi).
	widths := []float64{12, 22, 18, 28, 14, 10}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, w)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("laporan-penjualan-%s_%s.xlsx",
		from.Format("20060102"), to.Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ExportPenjualanPDF GET /laporan/penjualan/export.pdf.
func (h *LaporanHandler) ExportPenjualanPDF(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.collectPenjualanAll(c, from, to)
	if err != nil {
		return err
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 7, "LAPORAN PENJUALAN", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, fmt.Sprintf("Periode: %s s/d %s",
		from.Format("02 Jan 2006"), to.Format("02 Jan 2006")), "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Header tabel.
	cols := []struct {
		title string
		w     float64
		align string
	}{
		{"Tanggal", 25, "C"},
		{"Nomor", 50, "L"},
		{"Gudang", 45, "L"},
		{"Mitra", 80, "L"},
		{"Total", 35, "R"},
		{"Status", 42, "C"},
	}
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(229, 231, 235)
	for _, col := range cols {
		pdf.CellFormat(col.w, 7, col.title, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	var totalSum int64
	for _, r := range rows {
		pdf.CellFormat(cols[0].w, 6, r.Tanggal.Format("02-01-2006"), "1", 0, cols[0].align, false, 0, "")
		pdf.CellFormat(cols[1].w, 6, r.NomorKwitansi, "1", 0, cols[1].align, false, 0, "")
		pdf.CellFormat(cols[2].w, 6, truncatePDF(r.GudangNama, 28), "1", 0, cols[2].align, false, 0, "")
		pdf.CellFormat(cols[3].w, 6, truncatePDF(r.MitraNama, 50), "1", 0, cols[3].align, false, 0, "")
		pdf.CellFormat(cols[4].w, 6, format.Rupiah(r.Total), "1", 0, cols[4].align, false, 0, "")
		pdf.CellFormat(cols[5].w, 6, strings.ToUpper(r.StatusBayar), "1", 0, cols[5].align, false, 0, "")
		pdf.Ln(-1)
		totalSum += r.Total
	}

	// Footer total.
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(cols[0].w+cols[1].w+cols[2].w+cols[3].w, 7, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(cols[4].w, 7, format.Rupiah(totalSum), "1", 0, "R", false, 0, "")
	pdf.CellFormat(cols[5].w, 7, fmt.Sprintf("%d trx", len(rows)), "1", 0, "C", false, 0, "")
	pdf.Ln(-1)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("laporan-penjualan-%s_%s.pdf",
		from.Format("20060102"), to.Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`inline; filename="%s"`, filename))
	return c.Blob(http.StatusOK, "application/pdf", buf.Bytes())
}

// collectPenjualanAll loop semua halaman utk dipakai exporter.
func (h *LaporanHandler) collectPenjualanAll(c echo.Context, from, to time.Time) ([]repo.LaporanPenjualanRow, error) {
	var gudangID, mitraID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}
	if m, _ := strconv.ParseInt(c.QueryParam("mitra_id"), 10, 64); m > 0 {
		mitraID = &m
	}
	filter := service.LaporanPenjualanFilter{
		From: from, To: to, GudangID: gudangID, MitraID: mitraID,
		Page: 1, PerPage: 200,
	}
	all := make([]repo.LaporanPenjualanRow, 0, 256)
	for {
		rows, total, err := h.laporan.Penjualan(c.Request().Context(), filter)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if filter.Page*filter.PerPage >= total || len(rows) == 0 {
			break
		}
		filter.Page++
	}
	return all, nil
}

func truncatePDF(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "."
}

// Mutasi GET /laporan/mutasi.
func (h *LaporanHandler) Mutasi(c echo.Context) error {
	from, to := parseRange(c)
	var gudangID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}

	rows, err := h.laporan.Mutasi(c.Request().Context(), service.LaporanMutasiFilter{
		From: from, To: to, GudangID: gudangID,
	})
	if err != nil {
		return err
	}

	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}

	gid := int64(0)
	if gudangID != nil {
		gid = *gudangID
	}

	props := laporan.MutasiProps{
		Nav:      layout.DefaultNav("/laporan"),
		User:     userData(auth.CurrentUser(c)),
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		Rows:     rows,
		GudangID: gid,
		Gudangs:  gudangs,
	}
	return RenderHTML(c, http.StatusOK, laporan.Mutasi(props))
}

// StokKritis GET /laporan/stok-kritis.
func (h *LaporanHandler) StokKritis(c echo.Context) error {
	rows, err := h.laporan.StokKritis(c.Request().Context())
	if err != nil {
		return err
	}
	props := laporan.StokKritisProps{
		Nav:  layout.DefaultNav("/laporan"),
		User: userData(auth.CurrentUser(c)),
		Rows: rows,
	}
	return RenderHTML(c, http.StatusOK, laporan.StokKritis(props))
}

// TopProduk GET /laporan/top-produk.
func (h *LaporanHandler) TopProduk(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.laporan.TopProduk(c.Request().Context(), from, to, 20)
	if err != nil {
		return err
	}
	props := laporan.TopProdukProps{
		Nav:  layout.DefaultNav("/laporan"),
		User: userData(auth.CurrentUser(c)),
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Rows: rows,
	}
	return RenderHTML(c, http.StatusOK, laporan.TopProduk(props))
}

// Cashflow GET /laporan/cashflow.
func (h *LaporanHandler) Cashflow(c echo.Context) error {
	if h.cashflow == nil {
		return echo.NewHTTPError(http.StatusNotFound, "modul cashflow belum aktif")
	}
	ctx := c.Request().Context()
	from, to := parseRange(c)

	var gudangID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}

	summary, err := h.cashflow.Summary(ctx, from, to, gudangID)
	if err != nil {
		return err
	}

	// Periode sebelumnya - same length.
	prevFrom := from.AddDate(0, 0, -int(to.Sub(from).Hours()/24)-1)
	prevTo := from.AddDate(0, 0, -1)
	prevSum, _ := h.cashflow.Summary(ctx, prevFrom, prevTo, gudangID)
	prevDelta := summary.NetCashflow - prevSum.NetCashflow

	topKat, err := h.cashflow.KategoriBreakdown(ctx, from, to, "keluar", gudangID, 5)
	if err != nil {
		return err
	}
	daily, err := h.cashflow.DailyTrend(ctx, from, to, gudangID)
	if err != nil {
		return err
	}

	listFilter := repo.ListCashflowFilter{
		From: &from, To: &to, GudangID: gudangID, Page: 1, PerPage: 100,
	}
	pageRes, err := h.cashflow.List(ctx, listFilter)
	if err != nil {
		return err
	}

	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}
	gid := int64(0)
	if gudangID != nil {
		gid = *gudangID
	}

	return RenderHTML(c, http.StatusOK, laporan.Cashflow(laporan.CashflowProps{
		Nav:         layout.DefaultNav("/laporan"),
		User:        userData(auth.CurrentUser(c)),
		From:        from.Format("2006-01-02"),
		To:          to.Format("2006-01-02"),
		GudangID:    gid,
		Gudangs:     gudangs,
		Summary:     summary,
		PrevDelta:   prevDelta,
		Items:       pageRes.Items,
		TopKategori: topKat,
		Daily:       daily,
	}))
}

// ----- helpers --------------------------------------------------------------

func parseRange(c echo.Context) (time.Time, time.Time) {
	preset := strings.TrimSpace(c.QueryParam("preset"))
	fromStr := strings.TrimSpace(c.QueryParam("from"))
	toStr := strings.TrimSpace(c.QueryParam("to"))
	fromStr, toStr = resolvePeriodePreset(preset, fromStr, toStr)

	now := time.Now()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := to.AddDate(0, 0, -29)
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}
	return from, to
}

// resolvePeriodePreset menerjemahkan query "preset" menjadi pasangan from/to
// (YYYY-MM-DD). Jika preset kosong / tidak dikenal, from & to dikembalikan apa
// adanya. Zona waktu yang dipakai Asia/Jakarta dengan fallback UTC.
func resolvePeriodePreset(preset, from, to string) (string, string) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	if preset == "" {
		return from, to
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	const layout = "2006-01-02"

	switch preset {
	case "today":
		s := today.Format(layout)
		return s, s
	case "yesterday":
		y := today.AddDate(0, 0, -1).Format(layout)
		return y, y
	case "this_week":
		// Senin sebagai awal minggu.
		offset := int(today.Weekday()) - 1
		if offset < 0 { // Minggu (=0) → 6 hari sejak Senin lalu.
			offset = 6
		}
		start := today.AddDate(0, 0, -offset)
		return start.Format(layout), today.Format(layout)
	case "this_month":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		return start.Format(layout), today.Format(layout)
	case "this_year":
		start := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, loc)
		return start.Format(layout), today.Format(layout)
	case "last_month":
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		end := first.AddDate(0, 0, -1)
		start := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, loc)
		return start.Format(layout), end.Format(layout)
	case "last_year":
		start := time.Date(today.Year()-1, 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(today.Year()-1, 12, 31, 0, 0, 0, 0, loc)
		return start.Format(layout), end.Format(layout)
	}
	return from, to
}

// ----- shared export helpers -----------------------------------------------

func newLandscapePDF(title string, from, to time.Time) *gofpdf.Fpdf {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 7, title, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, fmt.Sprintf("Periode: %s s/d %s",
		from.Format("02 Jan 2006"), to.Format("02 Jan 2006")), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	return pdf
}

func writePDFHeader(pdf *gofpdf.Fpdf, headers []string, widths []float64) {
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(229, 231, 235)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 9)
}

func sendPDF(c echo.Context, pdf *gofpdf.Fpdf, name string, from, to time.Time) error {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s-%s_%s.pdf", name,
		from.Format("20060102"), to.Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`inline; filename="%s"`, filename))
	return c.Blob(http.StatusOK, "application/pdf", buf.Bytes())
}

func sendPDFNoPeriod(c echo.Context, pdf *gofpdf.Fpdf, name string) error {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s-%s.pdf", name, time.Now().Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`inline; filename="%s"`, filename))
	return c.Blob(http.StatusOK, "application/pdf", buf.Bytes())
}

func newXLSXFile(sheet string, headers []string, widths []float64) (*excelize.File, error) {
	f := excelize.NewFile()
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E5E7EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	first, _ := excelize.CoordinatesToCellName(1, 1)
	last, _ := excelize.CoordinatesToCellName(len(headers), 1)
	_ = f.SetCellStyle(sheet, first, last, headerStyle)
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, w)
	}
	return f, nil
}

func sendXLSX(c echo.Context, f *excelize.File, name string, from, to time.Time) error {
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s-%s_%s.xlsx", name,
		from.Format("20060102"), to.Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func sendXLSXNoPeriod(c echo.Context, f *excelize.File, name string) error {
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s-%s.xlsx", name, time.Now().Format("20060102"))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ----- Laba Rugi exports ----------------------------------------------------

func (h *LaporanHandler) lrAggregate(ctx context.Context, from, to time.Time) ([]repo.LaporanLR, lrTotals, error) {
	rows, err := h.laporan.LR(ctx, from, to)
	if err != nil {
		return nil, lrTotals{}, err
	}
	var t lrTotals
	for _, r := range rows {
		t.Penjualan += r.Penjualan
		t.Pembelian += r.Pembelian
		t.GrossProfit += r.GrossProfit
		t.BiayaOps += r.BiayaOperasional
		t.NetIncome += r.NetIncome
	}
	return rows, t, nil
}

type lrTotals struct {
	Penjualan, Pembelian, GrossProfit, BiayaOps, NetIncome int64
}

// ExportLRPDF GET /laporan/lr/export.pdf.
func (h *LaporanHandler) ExportLRPDF(c echo.Context) error {
	from, to := parseRange(c)
	_, t, err := h.lrAggregate(c.Request().Context(), from, to)
	if err != nil {
		return err
	}

	pdf := newLandscapePDF("LAPORAN LABA RUGI", from, to)
	headers := []string{"Keterangan", "Nilai"}
	widths := []float64{180, 80}
	writePDFHeader(pdf, headers, widths)

	row := func(label string, val int64, bold bool) {
		if bold {
			pdf.SetFont("Arial", "B", 9)
		} else {
			pdf.SetFont("Arial", "", 9)
		}
		pdf.CellFormat(widths[0], 6, label, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, format.Rupiah(val), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	row("Pendapatan (Penjualan)", t.Penjualan, false)
	row("Harga Pokok Penjualan (HPP)", t.Pembelian, false)
	row("Laba Kotor", t.GrossProfit, true)
	row("Beban Operasional", t.BiayaOps, false)
	row("Laba Bersih", t.NetIncome, true)

	return sendPDF(c, pdf, "laporan-lr", from, to)
}

// ExportLRXLSX GET /laporan/lr/export.xlsx.
func (h *LaporanHandler) ExportLRXLSX(c echo.Context) error {
	from, to := parseRange(c)
	_, t, err := h.lrAggregate(c.Request().Context(), from, to)
	if err != nil {
		return err
	}

	sheet := "Laba Rugi"
	f, err := newXLSXFile(sheet, []string{"Keterangan", "Nilai"}, []float64{40, 20})
	if err != nil {
		return err
	}
	rows := []struct {
		label string
		val   int64
	}{
		{"Pendapatan (Penjualan)", t.Penjualan},
		{"Harga Pokok Penjualan (HPP)", t.Pembelian},
		{"Laba Kotor", t.GrossProfit},
		{"Beban Operasional", t.BiayaOps},
		{"Laba Bersih", t.NetIncome},
	}
	for i, r := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), r.label)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), r.val/100)
	}
	return sendXLSX(c, f, "laporan-lr", from, to)
}

// ----- Mutasi exports -------------------------------------------------------

func (h *LaporanHandler) mutasiRows(c echo.Context, from, to time.Time) ([]repo.LaporanMutasiRow, error) {
	var gudangID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}
	return h.laporan.Mutasi(c.Request().Context(), service.LaporanMutasiFilter{
		From: from, To: to, GudangID: gudangID,
	})
}

// ExportMutasiPDF GET /laporan/mutasi/export.pdf.
func (h *LaporanHandler) ExportMutasiPDF(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.mutasiRows(c, from, to)
	if err != nil {
		return err
	}

	pdf := newLandscapePDF("LAPORAN MUTASI ANTAR GUDANG", from, to)
	headers := []string{"Tgl", "No", "Asal", "Tujuan", "Item", "Nilai", "Status"}
	widths := []float64{22, 40, 50, 50, 20, 40, 35}
	writePDFHeader(pdf, headers, widths)

	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.Tanggal.Format("02-01-2006"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, truncatePDF(r.NomorMutasi, 24), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, truncatePDF(r.GudangAsal, 30), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, truncatePDF(r.GudangTujuan, 30), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6, strconv.Itoa(r.JumlahItem), "1", 0, "R", false, 0, "")
		nilai := int64(0)
		if r.TotalNilai != nil {
			nilai = *r.TotalNilai
		}
		pdf.CellFormat(widths[5], 6, format.Rupiah(nilai), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[6], 6, strings.ToUpper(r.Status), "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}
	return sendPDF(c, pdf, "laporan-mutasi", from, to)
}

// ExportMutasiXLSX GET /laporan/mutasi/export.xlsx.
func (h *LaporanHandler) ExportMutasiXLSX(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.mutasiRows(c, from, to)
	if err != nil {
		return err
	}

	sheet := "Mutasi"
	headers := []string{"Tanggal", "Nomor", "Asal", "Tujuan", "Jumlah Item", "Total Nilai", "Status"}
	widths := []float64{12, 22, 24, 24, 12, 14, 14}
	f, err := newXLSXFile(sheet, headers, widths)
	if err != nil {
		return err
	}
	for i, r := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), r.Tanggal.Format("2006-01-02"))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), r.NomorMutasi)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), r.GudangAsal)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), r.GudangTujuan)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), r.JumlahItem)
		nilai := int64(0)
		if r.TotalNilai != nil {
			nilai = *r.TotalNilai
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), nilai/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", i+2), r.Status)
	}
	return sendXLSX(c, f, "laporan-mutasi", from, to)
}

// ----- Stok Kritis exports --------------------------------------------------

// ExportStokKritisPDF GET /laporan/stok-kritis/export.pdf.
func (h *LaporanHandler) ExportStokKritisPDF(c echo.Context) error {
	rows, err := h.laporan.StokKritis(c.Request().Context())
	if err != nil {
		return err
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 7, "LAPORAN STOK KRITIS", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, fmt.Sprintf("Per: %s", time.Now().Format("02 Jan 2006")),
		"", 1, "C", false, 0, "")
	pdf.Ln(3)

	headers := []string{"Produk", "Gudang", "Stok", "Min", "Selisih"}
	widths := []float64{110, 60, 30, 30, 35}
	writePDFHeader(pdf, headers, widths)

	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, truncatePDF(r.ProdukNama, 70), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, truncatePDF(r.GudangNama, 36), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, fmt.Sprintf("%.2f %s", r.Qty, r.SatuanKode), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[3], 6, fmt.Sprintf("%.2f", r.StokMinimum), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 6, fmt.Sprintf("%.2f", r.Qty-r.StokMinimum), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
	return sendPDFNoPeriod(c, pdf, "laporan-stok-kritis")
}

// ExportStokKritisXLSX GET /laporan/stok-kritis/export.xlsx.
func (h *LaporanHandler) ExportStokKritisXLSX(c echo.Context) error {
	rows, err := h.laporan.StokKritis(c.Request().Context())
	if err != nil {
		return err
	}
	sheet := "Stok Kritis"
	headers := []string{"Produk", "Gudang", "Stok", "Min", "Selisih", "Satuan"}
	widths := []float64{36, 24, 12, 12, 12, 10}
	f, err := newXLSXFile(sheet, headers, widths)
	if err != nil {
		return err
	}
	for i, r := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), r.ProdukNama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), r.GudangNama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), r.Qty)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), r.StokMinimum)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), r.Qty-r.StokMinimum)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), r.SatuanKode)
	}
	return sendXLSXNoPeriod(c, f, "laporan-stok-kritis")
}

// ----- Top Produk exports ---------------------------------------------------

// ExportTopProdukPDF GET /laporan/top-produk/export.pdf.
func (h *LaporanHandler) ExportTopProdukPDF(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.laporan.TopProduk(c.Request().Context(), from, to, 20)
	if err != nil {
		return err
	}

	pdf := newLandscapePDF("LAPORAN TOP PRODUK", from, to)
	headers := []string{"Rank", "Produk", "Qty Terjual", "Total Nilai"}
	widths := []float64{20, 150, 50, 60}
	writePDFHeader(pdf, headers, widths)

	var grand int64
	for i, r := range rows {
		pdf.CellFormat(widths[0], 6, strconv.Itoa(i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, truncatePDF(r.ProdukNama, 90), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, fmt.Sprintf("%.2f", r.QtyTotal), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[3], 6, format.Rupiah(r.Total), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
		grand += r.Total
	}
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(widths[0]+widths[1]+widths[2], 7, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[3], 7, format.Rupiah(grand), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	return sendPDF(c, pdf, "laporan-top-produk", from, to)
}

// ExportTopProdukXLSX GET /laporan/top-produk/export.xlsx.
func (h *LaporanHandler) ExportTopProdukXLSX(c echo.Context) error {
	from, to := parseRange(c)
	rows, err := h.laporan.TopProduk(c.Request().Context(), from, to, 20)
	if err != nil {
		return err
	}
	sheet := "Top Produk"
	headers := []string{"Rank", "Produk", "Qty Terjual", "Total Nilai"}
	widths := []float64{6, 40, 14, 18}
	f, err := newXLSXFile(sheet, headers, widths)
	if err != nil {
		return err
	}
	for i, r := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), r.ProdukNama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), r.QtyTotal)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), r.Total/100)
	}
	return sendXLSX(c, f, "laporan-top-produk", from, to)
}

// ----- Cashflow exports -----------------------------------------------------

func (h *LaporanHandler) cashflowItems(c echo.Context, from, to time.Time) ([]domain.Cashflow, error) {
	var gudangID *int64
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		gudangID = &g
	}
	all := make([]domain.Cashflow, 0, 256)
	page := 1
	for {
		res, err := h.cashflow.List(c.Request().Context(), repo.ListCashflowFilter{
			From: &from, To: &to, GudangID: gudangID, Page: page, PerPage: 200,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Items...)
		if page*200 >= res.Total || len(res.Items) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// ExportCashflowPDF GET /laporan/cashflow/export.pdf.
func (h *LaporanHandler) ExportCashflowPDF(c echo.Context) error {
	if h.cashflow == nil {
		return echo.NewHTTPError(http.StatusNotFound, "modul cashflow belum aktif")
	}
	from, to := parseRange(c)
	items, err := h.cashflowItems(c, from, to)
	if err != nil {
		return err
	}

	pdf := newLandscapePDF("LAPORAN CASHFLOW", from, to)
	headers := []string{"Tgl", "Kategori", "Tipe", "Deskripsi", "Masuk", "Keluar", "Saldo"}
	widths := []float64{22, 40, 20, 75, 35, 35, 50}
	writePDFHeader(pdf, headers, widths)

	var saldo, totalMasuk, totalKeluar int64
	for _, it := range items {
		var masuk, keluar int64
		if it.Tipe == domain.CashflowMasuk {
			masuk = it.Jumlah
			saldo += it.Jumlah
			totalMasuk += it.Jumlah
		} else {
			keluar = it.Jumlah
			saldo -= it.Jumlah
			totalKeluar += it.Jumlah
		}
		pdf.CellFormat(widths[0], 6, it.Tanggal.Format("02-01-2006"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, truncatePDF(it.Kategori, 24), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, strings.ToUpper(string(it.Tipe)), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[3], 6, truncatePDF(it.Deskripsi, 46), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6, format.Rupiah(masuk), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[5], 6, format.Rupiah(keluar), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[6], 6, format.Rupiah(saldo), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(widths[0]+widths[1]+widths[2]+widths[3], 7, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[4], 7, format.Rupiah(totalMasuk), "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[5], 7, format.Rupiah(totalKeluar), "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[6], 7, format.Rupiah(saldo), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)

	return sendPDF(c, pdf, "laporan-cashflow", from, to)
}

// ExportCashflowXLSX GET /laporan/cashflow/export.xlsx.
func (h *LaporanHandler) ExportCashflowXLSX(c echo.Context) error {
	if h.cashflow == nil {
		return echo.NewHTTPError(http.StatusNotFound, "modul cashflow belum aktif")
	}
	from, to := parseRange(c)
	items, err := h.cashflowItems(c, from, to)
	if err != nil {
		return err
	}
	sheet := "Cashflow"
	headers := []string{"Tanggal", "Kategori", "Tipe", "Deskripsi", "Masuk", "Keluar", "Saldo"}
	widths := []float64{12, 22, 10, 36, 14, 14, 16}
	f, err := newXLSXFile(sheet, headers, widths)
	if err != nil {
		return err
	}
	var saldo int64
	for i, it := range items {
		var masuk, keluar int64
		if it.Tipe == domain.CashflowMasuk {
			masuk = it.Jumlah
			saldo += it.Jumlah
		} else {
			keluar = it.Jumlah
			saldo -= it.Jumlah
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), it.Tanggal.Format("2006-01-02"))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), it.Kategori)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), string(it.Tipe))
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), it.Deskripsi)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), masuk/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), keluar/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", i+2), saldo/100)
	}
	return sendXLSX(c, f, "laporan-cashflow", from, to)
}

func (h *LaporanHandler) gudangLite(c echo.Context) ([]laporan.GudangLite, error) {
	return h.gudangLiteCtx(c.Request().Context())
}

func (h *LaporanHandler) gudangLiteCtx(ctx context.Context) ([]laporan.GudangLite, error) {
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]laporan.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, laporan.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}


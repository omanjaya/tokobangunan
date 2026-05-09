package handler

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
)

// bulkResultItem - per-id result untuk respons BulkCancel.
type bulkResultItem struct {
	ID     int64  `json:"id"`
	Status string `json:"status"` // ok | skipped | failed
	Error  string `json:"error,omitempty"`
}

// BulkCancel POST /penjualan/bulk-cancel.
// Form: csrf_token, ids[]=N, alasan.
// Best-effort batch — invoice yang gagal dilaporkan per-id; sukses lainnya
// tetap committed (Cancel sudah atomic per invoice di service).
func (h *PenjualanHandler) BulkCancel(c echo.Context) error {
	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user tidak terautentikasi")
	}
	ids, err := bulkParseIDs(c)
	if err != nil {
		return err
	}
	alasan := strings.TrimSpace(c.FormValue("alasan"))

	ctx := c.Request().Context()
	results := make([]bulkResultItem, 0, len(ids))
	okCount, failCount, skipCount := 0, 0, 0

	for _, id := range ids {
		err := h.penjualan.Cancel(ctx, id, u.ID, alasan)
		switch {
		case err == nil:
			okCount++
			results = append(results, bulkResultItem{ID: id, Status: "ok"})
		case isAlreadyCancelled(err):
			skipCount++
			results = append(results, bulkResultItem{ID: id, Status: "skipped", Error: "sudah dibatalkan"})
		default:
			failCount++
			results = append(results, bulkResultItem{ID: id, Status: "failed", Error: humanizePenjualanError(err)})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":      okCount,
		"skipped": skipCount,
		"failed":  failCount,
		"items":   results,
	})
}

// BulkExportXLSX GET /penjualan/bulk/export.xlsx?ids=1,2,3
// Multi-sheet single-rows: 1 sheet "Invoice" dengan 1 baris per invoice
// (header + total). Untuk ekspor detail per item, pakai ekspor laporan.
func (h *PenjualanHandler) BulkExportXLSX(c echo.Context) error {
	ids, err := bulkParseIDsQuery(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Invoice"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	headers := []string{"Tanggal", "Nomor Kwitansi", "Gudang ID", "Mitra ID", "Subtotal", "Diskon", "PPN", "Total", "Status"}
	for i, head := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, head)
	}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E5E7EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	first, _ := excelize.CoordinatesToCellName(1, 1)
	last, _ := excelize.CoordinatesToCellName(len(headers), 1)
	_ = f.SetCellStyle(sheet, first, last, headerStyle)

	row := 2
	for _, id := range ids {
		pj, gerr := h.penjualan.Get(ctx, id)
		if gerr != nil || pj == nil {
			continue
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), pj.Tanggal.Format("2006-01-02"))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), pj.NomorKwitansi)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), pj.GudangID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), pj.MitraID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), pj.Subtotal/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), pj.Diskon/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), pj.PPNAmount/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), pj.Total/100)
		_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), string(pj.StatusBayar))
		row++
	}

	widths := []float64{12, 22, 10, 10, 14, 12, 12, 14, 12}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, w)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return err
	}
	filename := fmt.Sprintf("penjualan-bulk-%d.xlsx", len(ids))
	c.Response().Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// bulkParseIDs parse `ids[]` form values (post). Dedup + drop ids invalid.
func bulkParseIDs(c echo.Context) ([]int64, error) {
	form, err := c.FormParams()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "form invalid")
	}
	raw := form["ids[]"]
	if len(raw) == 0 {
		raw = form["ids"]
	}
	if len(raw) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "ids kosong")
	}
	return dedupParseInt64(raw), nil
}

// bulkParseIDsQuery parse query `ids=1,2,3` atau `ids=1&ids=2`.
func bulkParseIDsQuery(c echo.Context) ([]int64, error) {
	qp := c.QueryParams()
	raw := qp["ids"]
	if len(raw) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "ids kosong")
	}
	// Mungkin "1,2,3" dalam 1 entry.
	expanded := make([]string, 0, len(raw))
	for _, s := range raw {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				expanded = append(expanded, p)
			}
		}
	}
	if len(expanded) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "ids kosong")
	}
	return dedupParseInt64(expanded), nil
}

func dedupParseInt64(in []string) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(in))
	for _, s := range in {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func isAlreadyCancelled(err error) bool {
	return errors.Is(err, domain.ErrInvoiceDibatalkan)
}

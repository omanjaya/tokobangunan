package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/print/escpos"
	"github.com/omanjaya/tokobangunan/internal/print/pdf"
	"github.com/omanjaya/tokobangunan/internal/print/thermal"
)

// PrintPDF GET /penjualan/:id/print/pdf?copy=asli|tembusan.
func (h *PenjualanHandler) PrintPDF(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}

	watermark := strings.ToUpper(strings.TrimSpace(c.QueryParam("copy")))
	switch watermark {
	case "ASLI", "TEMBUSAN", "":
	default:
		watermark = ""
	}

	tokoInfo := h.resolveTokoInfo(ctx)
	bytesPDF, err := pdf.GenerateKwitansiA5(pj, mitra, gudang, tokoInfo, watermark)
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`inline; filename="kwitansi-`+sanitizeFilename(pj.NomorKwitansi)+`.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", bytesPDF)
}

// PrintFaktur GET /penjualan/:id/print/faktur.
// Generate Faktur Pajak PDF — hanya kalau penjualan memiliki PPN > 0.
func (h *PenjualanHandler) PrintFaktur(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	if pj.PPNAmount <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "penjualan ini tidak memiliki PPN")
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}

	tokoInfo := h.resolveTokoInfo(ctx)
	pajak := h.resolvePajakConfig(ctx)
	bytesPDF, err := pdf.GenerateFakturPajak(pj, mitra, gudang, pajak, tokoInfo)
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`inline; filename="faktur-pajak-`+sanitizeFilename(pj.NomorKwitansi)+`.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", bytesPDF)
}

// resolvePajakConfig ambil dari app_setting; nil-safe.
func (h *PenjualanHandler) resolvePajakConfig(ctx context.Context) *domain.PajakConfig {
	if h.appSetting == nil {
		return nil
	}
	cfg, err := h.appSetting.PajakConfig(ctx)
	if err != nil {
		return nil
	}
	return cfg
}

// PrintDotMatrix GET /penjualan/:id/print/escp.
func (h *PenjualanHandler) PrintDotMatrix(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}

	tokoInfo := h.resolveTokoInfo(ctx)
	raw, err := escpos.GenerateKwitansiESCP(pj, mitra, gudang, tokoInfo)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="kwitansi-`+sanitizeFilename(pj.NomorKwitansi)+`.prn"`)
	return c.Blob(http.StatusOK, "application/octet-stream", raw)
}

// PrintThermal58 GET /penjualan/:id/print/58mm.
func (h *PenjualanHandler) PrintThermal58(c echo.Context) error {
	return h.printThermal(c, 58)
}

// PrintThermal80 GET /penjualan/:id/print/80mm.
func (h *PenjualanHandler) PrintThermal80(c echo.Context) error {
	return h.printThermal(c, 80)
}

func (h *PenjualanHandler) printThermal(c echo.Context, mm int) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}

	tokoInfo := h.resolveTokoInfo(ctx)
	var raw []byte
	switch mm {
	case 58:
		raw, err = thermal.Generate58mm(pj, mitra, gudang, tokoInfo)
	default:
		raw, err = thermal.Generate80mm(pj, mitra, gudang, tokoInfo)
	}
	if err != nil {
		return err
	}
	suffix := "58mm"
	if mm == 80 {
		suffix = "80mm"
	}
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="kwitansi-`+sanitizeFilename(pj.NomorKwitansi)+`-`+suffix+`.bin"`)
	return c.Blob(http.StatusOK, "application/octet-stream", raw)
}

// resolveTokoInfo ambil info toko dari app_setting; nil-safe.
func (h *PenjualanHandler) resolveTokoInfo(ctx context.Context) *domain.TokoInfo {
	if h.appSetting == nil {
		return nil
	}
	t, err := h.appSetting.TokoInfo(ctx)
	if err != nil {
		return nil
	}
	return t
}

// sanitizeFilename ganti '/' jadi '-' supaya nomor kwitansi aman dipakai filename.
func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "-", "\\", "-", " ", "_")
	return r.Replace(s)
}

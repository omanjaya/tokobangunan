package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/print/pdf"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	portalview "github.com/omanjaya/tokobangunan/internal/view/portal"
)

// PortalHandler - public-facing customer portal mitra (no auth).
type PortalHandler struct {
	access     *service.MitraAccessService
	mitra      *service.MitraService
	gudang     *service.GudangService
	penjualan  *service.PenjualanService
	piutang    *service.PiutangService
	pembayaran *service.PembayaranService
	appSetting *service.AppSettingService
}

// NewPortalHandler konstruktor.
func NewPortalHandler(
	access *service.MitraAccessService,
	mitra *service.MitraService,
	gudang *service.GudangService,
	penjualan *service.PenjualanService,
	piutang *service.PiutangService,
	pembayaran *service.PembayaranService,
	appSetting *service.AppSettingService,
) *PortalHandler {
	return &PortalHandler{
		access: access, mitra: mitra, gudang: gudang, penjualan: penjualan,
		piutang: piutang, pembayaran: pembayaran, appSetting: appSetting,
	}
}

// Show GET /portal/:token - public portal page.
func (h *PortalHandler) Show(c echo.Context) error {
	token := c.Param("token")
	ctx := c.Request().Context()

	t, err := h.access.GetByToken(ctx, token)
	if err != nil {
		return h.renderTokenError(c, err)
	}
	mitra, err := h.mitra.Get(ctx, t.MitraID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "mitra tidak ditemukan")
	}

	_, sum, invs, _ := h.piutang.MitraDetail(ctx, mitra.ID)
	outstanding := int64(0)
	if sum != nil {
		outstanding = sum.Outstanding
	}

	// Recent payments (last 10).
	bayarPage, _ := h.pembayaran.ListByMitra(ctx, mitra.ID, repo.ListPembayaranFilter{Page: 1, PerPage: 10})

	tokoInfo, _ := h.appSetting.TokoInfo(ctx)
	tokoNama := "Toko"
	if tokoInfo != nil && tokoInfo.Nama != "" {
		tokoNama = tokoInfo.Nama
	}

	props := portalview.Props{
		Mitra:       mitra,
		TokoNama:    tokoNama,
		Token:       token,
		Outstanding: outstanding,
		Invoices:    invs,
		Pembayaran:  bayarPage.Items,
	}
	return RenderHTML(c, http.StatusOK, portalview.Show(props))
}

// DownloadPDF GET /portal/:token/penjualan/:id/pdf - public PDF (validate ownership).
func (h *PortalHandler) DownloadPDF(c echo.Context) error {
	token := c.Param("token")
	ctx := c.Request().Context()
	t, err := h.access.GetByToken(ctx, token)
	if err != nil {
		return h.renderTokenError(c, err)
	}
	id, err := pathID(c)
	if err != nil {
		return err
	}
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	// Validasi: penjualan harus milik mitra ini.
	if pj.MitraID != t.MitraID {
		return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}
	tokoInfo, _ := h.appSetting.TokoInfo(ctx)
	bytesPDF, err := pdf.GenerateKwitansiA5(pj, mitra, gudang, tokoInfo, "")
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`inline; filename="kwitansi-`+sanitizeFilename(pj.NomorKwitansi)+`.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", bytesPDF)
}

func (h *PortalHandler) renderTokenError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrAccessTokenNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "Link tidak valid")
	case errors.Is(err, domain.ErrAccessTokenExpired):
		return echo.NewHTTPError(http.StatusGone, "Link sudah kedaluwarsa")
	case errors.Is(err, domain.ErrAccessTokenRevoked):
		return echo.NewHTTPError(http.StatusForbidden, "Link sudah dicabut")
	default:
		return echo.NewHTTPError(http.StatusForbidden, "Link tidak valid")
	}
}

package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/dashboard"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

type DashboardHandler struct {
	laporan *service.LaporanService
}

// NewDashboardHandler menerima LaporanService untuk akses agregat dashboard.
// Wiring di main.go perlu disesuaikan.
func NewDashboardHandler(laporan *service.LaporanService) *DashboardHandler {
	return &DashboardHandler{laporan: laporan}
}

func (h *DashboardHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	props := dashboard.IndexProps{
		Nav: layout.DefaultNav("/dashboard"),
		User: layout.UserData{
			Name: user.NamaLengkap,
			Role: user.Role,
		},
	}

	// Above-fold only: KPI + sales 30 hari + top mitra. Section bawah
	// di-fetch via htmx setelah revealed (lihat dashboard/partials.templ
	// dan SectionXxx handler di bawah). Initial response lebih ringan,
	// LCP dijamin turun.
	if h.laporan != nil {
		if data, err := h.laporan.DashboardAboveFold(c.Request().Context(), user); err == nil {
			props.Data = data
		}
	}

	return RenderHTML(c, http.StatusOK, dashboard.Index(props))
}

// SectionStokKritis - lazy partial untuk card "Stok kritis".
func (h *DashboardHandler) SectionStokKritis(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.NoContent(http.StatusUnauthorized)
	}
	rows, err := h.laporan.DashboardStokKritis(c.Request().Context(), user)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return RenderHTML(c, http.StatusOK, dashboard.PartialStokKritis(rows))
}

// SectionRecentTrx - lazy partial untuk card "Transaksi terakhir".
func (h *DashboardHandler) SectionRecentTrx(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.NoContent(http.StatusUnauthorized)
	}
	rows, err := h.laporan.DashboardRecentTrx(c.Request().Context(), user)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return RenderHTML(c, http.StatusOK, dashboard.PartialRecentTrx(rows))
}

// SectionRecentPembayaran - lazy partial untuk card "Pembayaran terakhir".
func (h *DashboardHandler) SectionRecentPembayaran(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.NoContent(http.StatusUnauthorized)
	}
	rows, err := h.laporan.DashboardRecentPembayaran(c.Request().Context())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return RenderHTML(c, http.StatusOK, dashboard.PartialRecentPembayaran(rows))
}

// SectionRecentMutasi - lazy partial untuk card "Mutasi terakhir".
func (h *DashboardHandler) SectionRecentMutasi(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.NoContent(http.StatusUnauthorized)
	}
	rows, err := h.laporan.DashboardRecentMutasi(c.Request().Context())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return RenderHTML(c, http.StatusOK, dashboard.PartialRecentMutasi(rows))
}

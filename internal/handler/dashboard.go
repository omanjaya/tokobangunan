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

	if h.laporan != nil {
		if data, err := h.laporan.Dashboard(c.Request().Context(), user); err == nil {
			props.Data = data
		}
	}

	return RenderHTML(c, http.StatusOK, dashboard.Index(props))
}

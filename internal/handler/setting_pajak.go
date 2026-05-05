package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	settingpajak "github.com/omanjaya/tokobangunan/internal/view/setting/pajak"
)

// SettingPajakHandler - halaman /setting/pajak.
type SettingPajakHandler struct {
	appSetting *service.AppSettingService
}

func NewSettingPajakHandler(as *service.AppSettingService) *SettingPajakHandler {
	return &SettingPajakHandler{appSetting: as}
}

// Show GET /setting/pajak.
func (h *SettingPajakHandler) Show(c echo.Context) error {
	cfg, err := h.appSetting.PajakConfig(c.Request().Context())
	if err != nil {
		return err
	}
	return RenderHTML(c, http.StatusOK, settingpajak.Form(settingpajak.Props{
		Nav:    layout.DefaultNav("/setting"),
		User:   userData(auth.CurrentUser(c)),
		Config: *cfg,
		OK:     c.QueryParam("ok") == "1",
	}))
}

// Update POST /setting/pajak.
func (h *SettingPajakHandler) Update(c echo.Context) error {
	cfg := &domain.PajakConfig{}
	cfg.PPNEnabled = parseCheckbox(c.FormValue("ppn_enabled"))
	cfg.PKP = parseCheckbox(c.FormValue("pkp"))
	if v := strings.TrimSpace(c.FormValue("ppn_persen")); v != "" {
		f, _ := strconv.ParseFloat(v, 64)
		cfg.PPNPersen = f
	}
	cfg.NamaPKP = c.FormValue("nama_pkp")
	cfg.AlamatPKP = c.FormValue("alamat_pkp")
	cfg.NPWPPKP = c.FormValue("npwp_pkp")

	var userIDPtr *int64
	if u := auth.CurrentUser(c); u != nil {
		uid := u.ID
		userIDPtr = &uid
	}
	if err := h.appSetting.UpdatePajakConfig(c.Request().Context(), cfg, userIDPtr); err != nil {
		return RenderHTML(c, http.StatusUnprocessableEntity, settingpajak.Form(settingpajak.Props{
			Nav:    layout.DefaultNav("/setting"),
			User:   userData(auth.CurrentUser(c)),
			Config: *cfg,
			Error:  err.Error(),
		}))
	}
	return c.Redirect(http.StatusSeeOther, "/setting/pajak?ok=1")
}

func parseCheckbox(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "on" || v == "1" || v == "true"
}

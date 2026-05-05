package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	satuanview "github.com/omanjaya/tokobangunan/internal/view/satuan"
)

// SatuanHandler - HTTP handler master satuan.
type SatuanHandler struct {
	svc *service.SatuanService
}

func NewSatuanHandler(s *service.SatuanService) *SatuanHandler {
	return &SatuanHandler{svc: s}
}

// Index GET /satuan - list + form inline.
func (h *SatuanHandler) Index(c echo.Context) error {
	return h.render(c, http.StatusOK, satuanview.IndexProps{})
}

// Create POST /satuan.
func (h *SatuanHandler) Create(c echo.Context) error {
	var in dto.SatuanCreateInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if _, err := h.svc.Create(c.Request().Context(), in); err != nil {
		props := satuanview.IndexProps{Input: in}
		if fes, ok := dto.CollectFieldErrors(err); ok {
			props.Errors = fes
		} else {
			props.General = humanizeSatuanError(err)
		}
		return h.render(c, http.StatusUnprocessableEntity, props)
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/satuan")
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, "/satuan")
}

func (h *SatuanHandler) render(c echo.Context, status int, p satuanview.IndexProps) error {
	items, err := h.svc.List(c.Request().Context())
	if err != nil {
		return err
	}
	p.Items = items
	p.Nav = layout.DefaultNav("/satuan")
	p.User = userData(auth.CurrentUser(c))
	return RenderHTML(c, status, satuanview.Index(p))
}

func humanizeSatuanError(err error) string {
	switch {
	case errors.Is(err, domain.ErrSatuanKodeDuplikat):
		return "Kode satuan sudah dipakai."
	case errors.Is(err, domain.ErrSatuanKodeWajib):
		return "Kode satuan wajib diisi."
	case errors.Is(err, domain.ErrSatuanKodeFormat):
		return "Kode harus huruf kecil tanpa spasi."
	case errors.Is(err, domain.ErrSatuanNamaWajib):
		return "Nama satuan wajib diisi."
	default:
		return "Gagal menyimpan satuan: " + err.Error()
	}
}

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
	produkview "github.com/omanjaya/tokobangunan/internal/view/produk"
)

// HargaHandler - HTTP handler harga per produk.
type HargaHandler struct {
	produk *service.ProdukService
	harga  *service.HargaService
}

func NewHargaHandler(p *service.ProdukService, h *service.HargaService) *HargaHandler {
	return &HargaHandler{produk: p, harga: h}
}

// Index GET /produk/:id/harga.
func (h *HargaHandler) Index(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	return h.render(c, http.StatusOK, id, produkview.HargaProps{})
}

// Set POST /produk/:id/harga.
func (h *HargaHandler) Set(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var in dto.HargaSetInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if _, err := h.harga.SetHarga(c.Request().Context(), id, in); err != nil {
		props := produkview.HargaProps{Input: in}
		if fes, ok := dto.CollectFieldErrors(err); ok {
			props.Errors = fes
		} else {
			props.General = humanizeHargaError(err)
		}
		return h.render(c, http.StatusUnprocessableEntity, id, props)
	}
	return c.Redirect(http.StatusSeeOther, "/produk/"+c.Param("id")+"/harga")
}

func (h *HargaHandler) render(c echo.Context, status int, produkID int64, p produkview.HargaProps) error {
	ctx := c.Request().Context()
	prod, err := h.produk.Get(ctx, produkID)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}
	hist, err := h.harga.ListByProduk(ctx, produkID)
	if err != nil {
		return err
	}
	p.Produk = *prod
	p.History = hist
	p.Nav = layout.DefaultNav("/produk")
	p.User = userData(auth.CurrentUser(c))
	return RenderHTML(c, status, produkview.Harga(p))
}

func humanizeHargaError(err error) string {
	switch {
	case errors.Is(err, domain.ErrHargaJualInvalid):
		return "Harga jual harus lebih besar dari 0."
	case errors.Is(err, domain.ErrHargaTipeInvalid):
		return "Tipe harga harus eceran, grosir, atau proyek."
	case errors.Is(err, domain.ErrHargaTanggalInvalid):
		return "Tanggal berlaku tidak valid (format YYYY-MM-DD)."
	case errors.Is(err, domain.ErrProdukNotFound):
		return "Produk tidak ditemukan."
	default:
		return "Gagal menyimpan harga: " + err.Error()
	}
}

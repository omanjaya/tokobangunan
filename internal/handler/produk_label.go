package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/print/pdf"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// ProdukLabelHandler - handler cetak label barcode produk.
type ProdukLabelHandler struct {
	produk *service.ProdukService
	harga  *service.HargaService
}

func NewProdukLabelHandler(p *service.ProdukService, h *service.HargaService) *ProdukLabelHandler {
	return &ProdukLabelHandler{produk: p, harga: h}
}

// LabelPDF GET /produk/:id/label?count=10 - download PDF label barcode.
func (h *ProdukLabelHandler) LabelPDF(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.produk.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}

	count, _ := strconv.Atoi(c.QueryParam("count"))
	if count <= 0 {
		count = 10
	}

	info := pdf.LabelInfo{SKU: p.SKU, Nama: p.Nama}
	if hak, herr := h.harga.GetAktif(ctx, p.ID, nil, domain.TipeHargaEceran); herr == nil {
		info.HargaCent = hak.HargaJual
	}

	data, err := pdf.GenerateLabelProdukPDF(p, info, count)
	if err != nil {
		return fmt.Errorf("generate label pdf: %w", err)
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="label-%s.pdf"`, p.SKU))
	return c.Blob(http.StatusOK, "application/pdf", data)
}

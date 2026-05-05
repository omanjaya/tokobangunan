package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	stokview "github.com/omanjaya/tokobangunan/internal/view/stok"
)

// StokHandler - HTTP handler modul stok.
type StokHandler struct {
	stok   *service.StokService
	produk *service.ProdukService
	gudang *service.GudangService
}

func NewStokHandler(s *service.StokService, p *service.ProdukService, g *service.GudangService) *StokHandler {
	return &StokHandler{stok: s, produk: p, gudang: g}
}

// Index GET /stok - landing dengan tab gudang. Default: gudang user, atau
// gudang pertama bila user owner/admin tanpa gudang_default.
func (h *StokHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}
	if len(gudangs) == 0 {
		props := stokview.IndexProps{
			Nav:  layout.DefaultNav("/stok"),
			User: userData(auth.CurrentUser(c)),
		}
		return RenderHTML(c, http.StatusOK, stokview.Index(props))
	}

	user := auth.CurrentUser(c)
	target := gudangs[0].ID
	if user != nil && user.GudangID != nil {
		for _, g := range gudangs {
			if g.ID == *user.GudangID {
				target = g.ID
				break
			}
		}
	}
	return c.Redirect(http.StatusSeeOther, "/stok/"+strconv.FormatInt(target, 10))
}

// Detail GET /stok/:gudang_id
func (h *StokHandler) Detail(c echo.Context) error {
	gudangID, err := strconv.ParseInt(c.Param("gudang_id"), 10, 64)
	if err != nil || gudangID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "gudang_id tidak valid")
	}
	ctx := c.Request().Context()

	if _, err := h.gudang.Get(ctx, gudangID); err != nil {
		if errors.Is(err, domain.ErrGudangNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "gudang tidak ditemukan")
		}
		return err
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	kategori := strings.TrimSpace(c.QueryParam("kategori"))
	low := c.QueryParam("low") == "1"
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	filter := repo.ListStokFilter{
		Query:        q,
		LowStockOnly: low,
		Page:         page,
		PerPage:      25,
	}
	if kategori != "" {
		filter.Kategori = &kategori
	}

	res, err := h.stok.ListByGudang(ctx, gudangID, filter)
	if err != nil {
		return err
	}

	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}
	kategoris, err := h.produk.ListKategori(ctx)
	if err != nil {
		return err
	}

	low2, err := h.stok.ListLowStock(ctx, &gudangID)
	if err != nil {
		return err
	}

	props := stokview.IndexProps{
		Nav:            layout.DefaultNav("/stok"),
		User:           userData(auth.CurrentUser(c)),
		Gudangs:        gudangs,
		ActiveGudangID: gudangID,
		Total:          res.Total,
		Page:           res.Page,
		PerPage:        res.PerPage,
		TotalPages:     res.TotalPages,
		Rows:           res.Items,
		Query:          q,
		Kategori:       kategori,
		LowStockOnly:   low,
		Kategoris:      kategoris,
		LowStockCount:  len(low2),
	}
	return RenderHTML(c, http.StatusOK, stokview.Index(props))
}

// RefreshAjax GET /stok/refresh?gudang_id=X - return partial tabel stok untuk
// auto-refresh HTMX (tanpa AppShell).
func (h *StokHandler) RefreshAjax(c echo.Context) error {
	gudangID, err := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	if err != nil || gudangID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "gudang_id tidak valid")
	}
	ctx := c.Request().Context()

	if _, err := h.gudang.Get(ctx, gudangID); err != nil {
		if errors.Is(err, domain.ErrGudangNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "gudang tidak ditemukan")
		}
		return err
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	kategori := strings.TrimSpace(c.QueryParam("kategori"))
	low := c.QueryParam("low") == "1"
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	filter := repo.ListStokFilter{
		Query:        q,
		LowStockOnly: low,
		Page:         page,
		PerPage:      25,
	}
	if kategori != "" {
		filter.Kategori = &kategori
	}

	res, err := h.stok.ListByGudang(ctx, gudangID, filter)
	if err != nil {
		return err
	}

	props := stokview.RefreshPartialProps{
		GudangID:     gudangID,
		Rows:         res.Items,
		Total:        res.Total,
		Page:         res.Page,
		TotalPages:   res.TotalPages,
		Query:        q,
		Kategori:     kategori,
		LowStockOnly: low,
		UpdatedAt:    time.Now().Format("15:04:05"),
	}
	return RenderHTML(c, http.StatusOK, stokview.RefreshPartial(props))
}

// Produk GET /stok/produk/:id - detail 1 produk di semua gudang.
func (h *StokHandler) Produk(c echo.Context) error {
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
	rows, err := h.stok.Snapshot(ctx, id)
	if err != nil {
		return err
	}
	props := stokview.ProdukDetailProps{
		Nav:        layout.DefaultNav("/stok"),
		User:       userData(auth.CurrentUser(c)),
		ProdukID:   p.ID,
		ProdukNama: p.Nama,
		ProdukSKU:  p.SKU,
		Rows:       rows,
	}
	return RenderHTML(c, http.StatusOK, stokview.ProdukDetail(props))
}

// ----- helpers ---------------------------------------------------------------

func (h *StokHandler) gudangLite(ctx context.Context) ([]stokview.GudangLite, error) {
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]stokview.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, stokview.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

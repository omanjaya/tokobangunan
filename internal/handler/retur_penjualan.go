package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	returview "github.com/omanjaya/tokobangunan/internal/view/retur_penjualan"
)

// ReturPenjualanHandler - HTTP handler modul retur penjualan.
type ReturPenjualanHandler struct {
	retur     *service.ReturPenjualanService
	penjualan *service.PenjualanService
	mitra     *service.MitraService
}

func NewReturPenjualanHandler(
	rs *service.ReturPenjualanService,
	pj *service.PenjualanService,
	mr *service.MitraService,
) *ReturPenjualanHandler {
	return &ReturPenjualanHandler{retur: rs, penjualan: pj, mitra: mr}
}

// RegisterReturRoutes register routes /retur-penjualan.
func RegisterReturRoutes(g *echo.Group, h *ReturPenjualanHandler) {
	rt := g.Group("/retur-penjualan")
	rt.GET("", h.Index)
	rt.GET("/baru", h.New)
	rt.POST("", h.Create)
	rt.GET("/:id", h.Show)
}

// Index GET /retur-penjualan
func (h *ReturPenjualanHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	filter := repo.ListReturPenjualanFilter{Page: page, PerPage: 25}
	if from := strings.TrimSpace(c.QueryParam("from")); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			filter.From = &t
		}
	}
	if to := strings.TrimSpace(c.QueryParam("to")); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			filter.To = &t
		}
	}
	result, err := h.retur.List(ctx, filter)
	if err != nil {
		return err
	}
	props := returview.IndexProps{
		Nav:        layout.DefaultNav("/retur-penjualan"),
		User:       penjualanUserData(c),
		Rows:       result.Items,
		Total:      result.Total,
		Page:       result.Page,
		PerPage:    result.PerPage,
		TotalPages: result.TotalPages,
		From:       c.QueryParam("from"),
		To:         c.QueryParam("to"),
	}
	return RenderHTML(c, http.StatusOK, returview.Index(props))
}

// New GET /retur-penjualan/baru?from=<penjualan_id>
func (h *ReturPenjualanHandler) New(c echo.Context) error {
	ctx := c.Request().Context()
	fromID, _ := strconv.ParseInt(c.QueryParam("from"), 10, 64)
	if fromID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "parameter from wajib")
	}
	pj, err := h.penjualan.Get(ctx, fromID)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	if pj.StatusBayar == domain.StatusBayarDibatalkan {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "invoice dibatalkan, tidak bisa diretur")
	}
	if err := enforceGudangScope(c, pj.GudangID); err != nil {
		return err
	}
	mitraNama := ""
	if m, err := h.mitra.Get(ctx, pj.MitraID); err == nil {
		mitraNama = m.Nama
	}
	props := returview.FormProps{
		Nav:           layout.DefaultNav("/retur-penjualan"),
		User:          penjualanUserData(c),
		Penjualan:     pj,
		MitraNama:     mitraNama,
		Tanggal:       time.Now().Format("2006-01-02"),
		CSRFToken:     csrfFromContext(c),
	}
	return RenderHTML(c, http.StatusOK, returview.Form(props))
}

// Create POST /retur-penjualan
func (h *ReturPenjualanHandler) Create(c echo.Context) error {
	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
	}
	if err := c.Request().ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "form invalid")
	}
	form := c.Request().PostForm
	in := dto.ReturPenjualanInput{
		Tanggal: strings.TrimSpace(form.Get("tanggal")),
		Alasan:  strings.TrimSpace(form.Get("alasan")),
		Catatan: strings.TrimSpace(form.Get("catatan")),
	}
	in.PenjualanID, _ = strconv.ParseInt(form.Get("penjualan_id"), 10, 64)

	// Items disusun array indexed: items[0][penjualan_item_id]=..., items[0][qty]=..., items[0][satuan_id]=...
	for i := 0; i < 100; i++ {
		prefix := "items[" + strconv.Itoa(i) + "]"
		pidStr := form.Get(prefix + "[penjualan_item_id]")
		if pidStr == "" {
			break
		}
		pid, _ := strconv.ParseInt(pidStr, 10, 64)
		qty, _ := strconv.ParseFloat(form.Get(prefix+"[qty]"), 64)
		sid, _ := strconv.ParseInt(form.Get(prefix+"[satuan_id]"), 10, 64)
		if qty <= 0 {
			continue
		}
		in.Items = append(in.Items, dto.ReturItemInput{
			PenjualanItemID: pid, Qty: qty, SatuanID: sid,
		})
	}

	r, err := h.retur.Create(c.Request().Context(), in, u.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	target := "/retur-penjualan/" + strconv.FormatInt(r.ID, 10)
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", target)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// Show GET /retur-penjualan/:id
func (h *ReturPenjualanHandler) Show(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	r, err := h.retur.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrReturPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "retur tidak ditemukan")
		}
		return err
	}
	if err := enforceGudangScope(c, r.GudangID); err != nil {
		return err
	}
	pj, _ := h.penjualan.Get(ctx, r.PenjualanID)
	mitraNama := ""
	if r.MitraID != nil {
		if m, err := h.mitra.Get(ctx, *r.MitraID); err == nil {
			mitraNama = m.Nama
		}
	}
	props := returview.ShowProps{
		Nav:       layout.DefaultNav("/retur-penjualan"),
		User:      penjualanUserData(c),
		Retur:     r,
		Penjualan: pj,
		MitraNama: mitraNama,
	}
	return RenderHTML(c, http.StatusOK, returview.Show(props))
}

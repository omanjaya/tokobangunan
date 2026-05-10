package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/laporan"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// ForecastHandler - HTTP handler inventory forecasting.
type ForecastHandler struct {
	forecast *service.ForecastService
	gudang   *service.GudangService
}

func NewForecastHandler(fs *service.ForecastService, gs *service.GudangService) *ForecastHandler {
	return &ForecastHandler{forecast: fs, gudang: gs}
}

// Index GET /laporan/reorder.
func (h *ForecastHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()

	lookback, _ := strconv.Atoi(c.QueryParam("lookback"))
	if lookback != 7 && lookback != 30 && lookback != 90 {
		lookback = 30
	}

	var gudangID *int64
	gid, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	if gid > 0 {
		gudangID = &gid
	}

	rows, err := h.forecast.Velocity(ctx, lookback, gudangID)
	if err != nil {
		return err
	}

	gudangs, err := h.gudangLiteCtx(ctx)
	if err != nil {
		return err
	}

	props := laporan.ReorderProps{
		Nav:          layout.DefaultNav("/laporan"),
		User:         userData(auth.CurrentUser(c)),
		GudangID:     gid,
		LookbackDays: lookback,
		Rows:         rows,
		Gudangs:      gudangs,
	}
	return RenderHTML(c, http.StatusOK, laporan.Reorder(props))
}

func (h *ForecastHandler) gudangLiteCtx(ctx context.Context) ([]laporan.GudangLite, error) {
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]laporan.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, laporan.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

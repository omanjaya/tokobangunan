package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	piutangview "github.com/omanjaya/tokobangunan/internal/view/piutang"
)

// PiutangHandler menangani routes /piutang/*.
type PiutangHandler struct {
	svc *service.PiutangService
}

// NewPiutangHandler konstruktor.
func NewPiutangHandler(svc *service.PiutangService) *PiutangHandler {
	return &PiutangHandler{svc: svc}
}

// Index GET /piutang - list mitra dengan piutang + summary aging.
func (h *PiutangHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	if redir, t := filterPersist(c, "tb_filter_piutang"); redir {
		return c.Redirect(http.StatusSeeOther, t)
	}
	ctx := c.Request().Context()

	q := strings.TrimSpace(c.QueryParam("q"))
	aging := strings.TrimSpace(c.QueryParam("aging"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	f := repo.ListPiutangFilter{Query: q, Page: page, PerPage: 25}
	if aging != "" {
		f.Aging = &aging
	}

	// Parallelize Summary (heavy FIFO aggregate) + AgingBuckets.
	var (
		res     service.PageResult[domain.PiutangSummary]
		buckets map[domain.PiutangAging]int64
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var e error
		res, e = h.svc.Summary(gctx, f)
		if e != nil {
			slog.ErrorContext(gctx, "list piutang failed", "error", e)
		}
		return e
	})
	g.Go(func() error {
		var e error
		buckets, e = h.svc.AgingBuckets(gctx)
		if e != nil {
			slog.ErrorContext(gctx, "aging buckets failed", "error", e)
		}
		return e
	})
	if err := g.Wait(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat data piutang")
	}

	props := piutangview.IndexProps{
		Nav:        layout.DefaultNav("/piutang"),
		User:       userData(user),
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		Query:      q,
		Aging:      aging,
		Buckets:    buckets,
	}
	return RenderHTML(c, http.StatusOK, piutangview.Index(props))
}

// MitraDetail GET /piutang/:mitra_id - detail invoice + form pembayaran.
func (h *PiutangHandler) MitraDetail(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("mitra_id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	ctx := c.Request().Context()

	m, sum, invs, err := h.svc.MitraDetail(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
	}

	csrf, _ := c.Get("csrf").(string)
	props := piutangview.MitraDetailProps{
		Nav:       layout.DefaultNav("/piutang"),
		User:      userData(user),
		Mitra:     m,
		Summary:   sum,
		Invoices:  invs,
		CSRFToken: csrf,
	}
	return RenderHTML(c, http.StatusOK, piutangview.MitraDetail(props))
}

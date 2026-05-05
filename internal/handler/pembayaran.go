package handler

import (
	"errors"
	"log/slog"
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
	pembayaranview "github.com/omanjaya/tokobangunan/internal/view/pembayaran"
)

// PembayaranHandler menangani routes /pembayaran/* + /mitra/:id/pembayaran.
type PembayaranHandler struct {
	svc       *service.PembayaranService
	mitraSvc  *service.MitraService
	piutang   *service.PiutangService
}

// NewPembayaranHandler konstruktor.
func NewPembayaranHandler(
	svc *service.PembayaranService,
	mitraSvc *service.MitraService,
	piutang *service.PiutangService,
) *PembayaranHandler {
	return &PembayaranHandler{svc: svc, mitraSvc: mitraSvc, piutang: piutang}
}

// Record POST /pembayaran - catat pembayaran (penjualan_id optional).
func (h *PembayaranHandler) Record(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	in := dto.PembayaranCreateInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	if err := dto.Validate(&in); err != nil {
		_, _ = dto.CollectFieldErrors(err)
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Validasi gagal: pastikan field terisi.")
	}

	p, err := h.svc.Record(ctx, in, user.ID)
	if err != nil {
		slog.ErrorContext(ctx, "record pembayaran failed", "error", err)
		if errors.Is(err, domain.ErrJumlahLebihDariOutstanding) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Jumlah melebihi outstanding invoice.")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mencatat pembayaran")
	}

	// Redirect ke detail piutang mitra.
	return c.Redirect(http.StatusSeeOther, "/piutang/"+strconv.FormatInt(p.MitraID, 10))
}

// RecordBatch POST /pembayaran/batch - alokasi FIFO ke invoice tertua.
func (h *PembayaranHandler) RecordBatch(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	in := dto.PembayaranBatchInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	if err := dto.Validate(&in); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Validasi gagal")
	}

	if _, err := h.svc.RecordBatch(ctx, in, user.ID); err != nil {
		slog.ErrorContext(ctx, "batch pembayaran failed", "error", err)
		if errors.Is(err, domain.ErrJumlahLebihDariOutstanding) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Jumlah melebihi total piutang mitra.")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mencatat pembayaran")
	}
	return c.Redirect(http.StatusSeeOther, "/piutang/"+strconv.FormatInt(in.MitraID, 10))
}

// MitraHistory GET /mitra/:id/pembayaran - list history pembayaran 1 mitra.
func (h *PembayaranHandler) MitraHistory(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	ctx := c.Request().Context()

	m, err := h.mitraSvc.Get(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	from := strings.TrimSpace(c.QueryParam("from"))
	to := strings.TrimSpace(c.QueryParam("to"))
	f := repo.ListPembayaranFilter{Page: page, PerPage: 25}
	if t, err := time.Parse("2006-01-02", from); err == nil {
		f.From = &t
	}
	if t, err := time.Parse("2006-01-02", to); err == nil {
		f.To = &t
	}

	res, err := h.svc.ListByMitra(ctx, id, f)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat history pembayaran")
	}

	props := pembayaranview.HistoryProps{
		Nav:        layout.DefaultNav("/mitra"),
		User:       userData(user),
		Mitra:      m,
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		From:       from,
		To:         to,
	}
	return RenderHTML(c, http.StatusOK, pembayaranview.History(props))
}

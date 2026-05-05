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
	opnameview "github.com/omanjaya/tokobangunan/internal/view/opname"
)

// StokOpnameHandler menangani routes /opname/*.
type StokOpnameHandler struct {
	svc        *service.StokOpnameService
	gudangRepo *repo.GudangRepo
}

// NewStokOpnameHandler konstruktor.
func NewStokOpnameHandler(svc *service.StokOpnameService, gudangRepo *repo.GudangRepo) *StokOpnameHandler {
	return &StokOpnameHandler{svc: svc, gudangRepo: gudangRepo}
}

// Index GET /opname.
func (h *StokOpnameHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	gudangID, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	status := strings.TrimSpace(c.QueryParam("status"))

	f := repo.ListStokOpnameFilter{
		GudangID: gudangID,
		Status:   status,
		Page:     page,
		PerPage:  25,
	}
	res, err := h.svc.List(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, "list opname failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar opname")
	}
	gudangs, _ := h.gudangRepo.List(ctx, false)

	props := opnameview.IndexProps{
		Nav:        layout.DefaultNav("/opname"),
		User:       userData(user),
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		GudangID:   gudangID,
		Status:     status,
		Gudangs:    gudangs,
	}
	return RenderHTML(c, http.StatusOK, opnameview.Index(props))
}

// New GET /opname/baru.
func (h *StokOpnameHandler) New(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()
	gudangs, _ := h.gudangRepo.List(ctx, false)

	props := opnameview.FormProps{
		Nav:     layout.DefaultNav("/opname"),
		User:    userData(user),
		Gudangs: gudangs,
		Input: dto.StokOpnameCreateInput{
			Tanggal: time.Now().Format("2006-01-02"),
		},
	}
	return RenderHTML(c, http.StatusOK, opnameview.Form(props))
}

// Create POST /opname.
func (h *StokOpnameHandler) Create(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	in := dto.StokOpnameCreateInput{}
	if err := c.Bind(&in); err != nil {
		return h.renderFormErr(c, user, in, nil, "Form tidak valid.")
	}
	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderFormErr(c, user, in, fe, "")
	}

	o, err := h.svc.Create(ctx, in, user.ID)
	if err != nil {
		slog.ErrorContext(ctx, "create opname failed", "error", err)
		return h.renderFormErr(c, user, in, nil, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/opname/"+strconv.FormatInt(o.ID, 10))
}

// Show GET /opname/:id.
func (h *StokOpnameHandler) Show(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID opname tidak valid")
	}
	ctx := c.Request().Context()

	o, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrOpnameTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Opname tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat opname")
	}
	props := opnameview.ShowProps{
		Nav:    layout.DefaultNav("/opname"),
		User:   userData(user),
		Opname: o,
	}
	return RenderHTML(c, http.StatusOK, opnameview.Show(props))
}

// UpdateItem POST /opname/:id/item/:produk_id.
func (h *StokOpnameHandler) UpdateItem(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	_ = user
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID opname tidak valid")
	}
	produkID, err := strconv.ParseInt(c.Param("produk_id"), 10, 64)
	if err != nil || produkID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID produk tidak valid")
	}

	in := dto.StokOpnameItemInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	if err := dto.Validate(&in); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Qty fisik tidak valid")
	}

	if err := h.svc.UpdateItem(c.Request().Context(), id, produkID, in.QtyFisik, in.Keterangan); err != nil {
		switch {
		case errors.Is(err, domain.ErrOpnameTidakDitemukan):
			return echo.NewHTTPError(http.StatusNotFound, "Opname tidak ditemukan")
		case errors.Is(err, domain.ErrOpnameTransitionInvalid):
			return echo.NewHTTPError(http.StatusConflict, "Item tidak bisa diubah pada status ini")
		}
		slog.ErrorContext(c.Request().Context(), "update opname item failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyimpan item")
	}
	return c.Redirect(http.StatusSeeOther, "/opname/"+strconv.FormatInt(id, 10))
}

// Submit POST /opname/:id/submit (draft → selesai).
func (h *StokOpnameHandler) Submit(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	_ = user
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID opname tidak valid")
	}
	if err := h.svc.Submit(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrOpnameTransitionInvalid) {
			return echo.NewHTTPError(http.StatusConflict, "Status tidak dapat diubah")
		}
		slog.ErrorContext(c.Request().Context(), "submit opname failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal submit opname")
	}
	return c.Redirect(http.StatusSeeOther, "/opname/"+strconv.FormatInt(id, 10))
}

// Approve POST /opname/:id/approve (selesai → approved + adjust stok via trigger).
func (h *StokOpnameHandler) Approve(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID opname tidak valid")
	}
	if err := h.svc.Approve(c.Request().Context(), id, user.ID); err != nil {
		if errors.Is(err, domain.ErrOpnameTransitionInvalid) {
			return echo.NewHTTPError(http.StatusConflict, "Hanya opname status 'selesai' yang bisa di-approve")
		}
		slog.ErrorContext(c.Request().Context(), "approve opname failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal approve opname")
	}
	return c.Redirect(http.StatusSeeOther, "/opname/"+strconv.FormatInt(id, 10))
}

// --- helpers ---

func (h *StokOpnameHandler) renderFormErr(c echo.Context, user *auth.User, in dto.StokOpnameCreateInput, fe map[string]string, general string) error {
	gudangs, _ := h.gudangRepo.List(c.Request().Context(), false)
	props := opnameview.FormProps{
		Nav:     layout.DefaultNav("/opname"),
		User:    userData(user),
		Input:   in,
		Errors:  fe,
		General: general,
		Gudangs: gudangs,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, opnameview.Form(props))
}

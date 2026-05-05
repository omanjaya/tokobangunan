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
	tabunganview "github.com/omanjaya/tokobangunan/internal/view/tabungan"
)

// TabunganHandler menangani routes /mitra/:id/tabungan/*.
type TabunganHandler struct {
	svc      *service.TabunganService
	mitraSvc *service.MitraService
}

// NewTabunganHandler konstruktor.
func NewTabunganHandler(svc *service.TabunganService, mitraSvc *service.MitraService) *TabunganHandler {
	return &TabunganHandler{svc: svc, mitraSvc: mitraSvc}
}

// Show GET /mitra/:id/tabungan - saldo + history + form.
func (h *TabunganHandler) Show(c echo.Context) error {
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
	f := repo.ListTabunganFilter{Page: page, PerPage: 25}
	if t, err := time.Parse("2006-01-02", from); err == nil {
		f.From = &t
	}
	if t, err := time.Parse("2006-01-02", to); err == nil {
		f.To = &t
	}

	res, err := h.svc.History(ctx, id, f)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat history tabungan")
	}
	saldo, err := h.svc.Saldo(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat saldo")
	}

	csrf, _ := c.Get("csrf").(string)
	props := tabunganview.ShowProps{
		Nav:        layout.DefaultNav("/mitra"),
		User:       userData(user),
		Mitra:      m,
		Saldo:      saldo,
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		From:       from,
		To:         to,
		FlashMsg:   c.QueryParam("flash"),
		FlashErr:   c.QueryParam("err"),
		CSRFToken:  csrf,
	}
	return RenderHTML(c, http.StatusOK, tabunganview.Show(props))
}

// Setor POST /mitra/:id/tabungan/setor.
func (h *TabunganHandler) Setor(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	ctx := c.Request().Context()

	in := dto.TabunganSetorInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	in.MitraID = id
	if err := dto.Validate(&in); err != nil {
		return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "", "Form setor tidak valid."))
	}
	if _, err := h.svc.Setor(ctx, in, user.ID); err != nil {
		slog.ErrorContext(ctx, "setor tabungan failed", "error", err)
		return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "", "Gagal menyimpan setoran."))
	}
	return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "Setoran berhasil dicatat.", ""))
}

// Tarik POST /mitra/:id/tabungan/tarik.
func (h *TabunganHandler) Tarik(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	ctx := c.Request().Context()

	in := dto.TabunganTarikInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	in.MitraID = id
	if err := dto.Validate(&in); err != nil {
		return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "", "Form tarik tidak valid."))
	}
	if _, err := h.svc.Tarik(ctx, in, user.ID); err != nil {
		slog.ErrorContext(ctx, "tarik tabungan failed", "error", err)
		if errors.Is(err, domain.ErrTabunganSaldoKurang) {
			return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "", "Saldo tabungan tidak cukup."))
		}
		return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "", "Gagal menyimpan penarikan."))
	}
	return c.Redirect(http.StatusSeeOther, redirectTabungan(id, "Penarikan berhasil dicatat.", ""))
}

func redirectTabungan(id int64, flash, errMsg string) string {
	u := "/mitra/" + strconv.FormatInt(id, 10) + "/tabungan"
	q := []string{}
	if flash != "" {
		q = append(q, "flash="+flash)
	}
	if errMsg != "" {
		q = append(q, "err="+errMsg)
	}
	if len(q) > 0 {
		u += "?" + strings.Join(q, "&")
	}
	return u
}

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
	"github.com/omanjaya/tokobangunan/internal/view/kas"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// CashflowHandler - HTTP handler modul kas.
type CashflowHandler struct {
	cashflow *service.CashflowService
	gudang   *service.GudangService
}

func NewCashflowHandler(cs *service.CashflowService, gs *service.GudangService) *CashflowHandler {
	return &CashflowHandler{cashflow: cs, gudang: gs}
}

// Index GET /kas.
func (h *CashflowHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	from, to := parseRangeKas(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	filter := repo.ListCashflowFilter{
		From:    &from,
		To:      &to,
		Page:    page,
		PerPage: 25,
	}
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		filter.GudangID = &g
	}
	if t := strings.TrimSpace(c.QueryParam("tipe")); t == "masuk" || t == "keluar" {
		filter.Tipe = t
	}
	filter.Kategori = strings.TrimSpace(c.QueryParam("kategori"))

	pageRes, err := h.cashflow.List(ctx, filter)
	if err != nil {
		return err
	}
	summary, err := h.cashflow.Summary(ctx, from, to, filter.GudangID)
	if err != nil {
		return err
	}
	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}

	gid := int64(0)
	if filter.GudangID != nil {
		gid = *filter.GudangID
	}

	return RenderHTML(c, http.StatusOK, kas.Index(kas.IndexProps{
		Nav:        layout.DefaultNav("/kas"),
		User:       userData(auth.CurrentUser(c)),
		Items:      pageRes.Items,
		Total:      pageRes.Total,
		Page:       pageRes.Page,
		PerPage:    pageRes.PerPage,
		TotalPages: pageRes.TotalPages,
		From:       from.Format("2006-01-02"),
		To:         to.Format("2006-01-02"),
		GudangID:   gid,
		Tipe:       filter.Tipe,
		Kategori:   filter.Kategori,
		Summary:    summary,
		Gudangs:    gudangs,
	}))
}

// New GET /kas/baru?tipe=masuk|keluar.
func (h *CashflowHandler) New(c echo.Context) error {
	tipe := strings.TrimSpace(c.QueryParam("tipe"))
	if tipe != "keluar" {
		tipe = "masuk"
	}
	return h.renderForm(c, dto.CashflowCreateInput{
		Tanggal: time.Now().Format("2006-01-02"),
		Tipe:    tipe,
		Metode:  "tunai",
	}, "", nil, http.StatusOK)
}

// Create POST /kas.
func (h *CashflowHandler) Create(c echo.Context) error {
	in := dto.CashflowCreateInput{
		Tanggal:   strings.TrimSpace(c.FormValue("tanggal")),
		Tipe:      strings.TrimSpace(c.FormValue("tipe")),
		Kategori:  strings.TrimSpace(c.FormValue("kategori")),
		Deskripsi: strings.TrimSpace(c.FormValue("deskripsi")),
		Metode:    strings.TrimSpace(c.FormValue("metode")),
		Referensi: strings.TrimSpace(c.FormValue("referensi")),
		Catatan:   strings.TrimSpace(c.FormValue("catatan")),
	}
	if v := strings.TrimSpace(c.FormValue("jumlah")); v != "" {
		in.Jumlah, _ = strconv.ParseInt(v, 10, 64)
	}
	if g, _ := strconv.ParseInt(c.FormValue("gudang_id"), 10, 64); g > 0 {
		in.GudangID = &g
	}

	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user tidak terautentikasi")
	}
	created, err := h.cashflow.Create(c.Request().Context(), u.ID, in)
	if err != nil {
		fes, ok := dto.CollectFieldErrors(err)
		var general string
		if !ok {
			general = humanizeCashflow(err)
		}
		return h.renderForm(c, in, general, fes, http.StatusUnprocessableEntity)
	}
	return c.Redirect(http.StatusSeeOther, "/kas/"+strconv.FormatInt(created.ID, 10))
}

// Show GET /kas/:id.
func (h *CashflowHandler) Show(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	cf, err := h.cashflow.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrCashflowNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "kas tidak ditemukan")
		}
		return err
	}
	var gudang *domain.Gudang
	if cf.GudangID != nil {
		if g, err := h.gudang.Get(ctx, *cf.GudangID); err == nil {
			gudang = g
		}
	}
	u := auth.CurrentUser(c)
	isOwner := u != nil && u.Role == "owner"
	return RenderHTML(c, http.StatusOK, kas.Show(kas.ShowProps{
		Nav:      layout.DefaultNav("/kas"),
		User:     userData(u),
		Cashflow: cf,
		Gudang:   gudang,
		IsOwner:  isOwner,
	}))
}

// Delete POST /kas/:id/delete (owner only).
func (h *CashflowHandler) Delete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.cashflow.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrCashflowNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "kas tidak ditemukan")
		}
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/kas")
}

// ----- helpers ---------------------------------------------------------------

func (h *CashflowHandler) renderForm(c echo.Context, in dto.CashflowCreateInput, general string, fes dto.FieldErrors, status int) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}
	katIn, _ := h.cashflow.ListKategori(ctx, domain.CashflowMasuk)
	katOut, _ := h.cashflow.ListKategori(ctx, domain.CashflowKeluar)
	return RenderHTML(c, status, kas.Form(kas.FormProps{
		Nav:         layout.DefaultNav("/kas"),
		User:        userData(auth.CurrentUser(c)),
		Input:       in,
		General:     general,
		Errors:      fes,
		Gudangs:     gudangs,
		KategoriIn:  katIn,
		KategoriOut: katOut,
	}))
}

func (h *CashflowHandler) gudangLite(c echo.Context) ([]kas.GudangLite, error) {
	list, err := h.gudang.List(c.Request().Context(), false)
	if err != nil {
		return nil, err
	}
	out := make([]kas.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, kas.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

func parseRangeKas(c echo.Context) (time.Time, time.Time) {
	now := time.Now()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := to.AddDate(0, 0, -29)
	if s := strings.TrimSpace(c.QueryParam("from")); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = t
		}
	}
	if s := strings.TrimSpace(c.QueryParam("to")); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = t
		}
	}
	return from, to
}

func humanizeCashflow(err error) string {
	switch {
	case errors.Is(err, domain.ErrCashflowJumlahInvalid):
		return "Jumlah harus lebih dari 0."
	case errors.Is(err, domain.ErrCashflowTipeInvalid):
		return "Tipe kas tidak valid."
	case errors.Is(err, domain.ErrCashflowKategoriWajib):
		return "Kategori wajib diisi."
	default:
		return "Gagal menyimpan kas: " + err.Error()
	}
}

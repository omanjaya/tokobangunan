package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	gudangview "github.com/omanjaya/tokobangunan/internal/view/setting/gudang"
)

type GudangHandler struct {
	svc *service.GudangService
}

func NewGudangHandler(svc *service.GudangService) *GudangHandler {
	return &GudangHandler{svc: svc}
}

func (h *GudangHandler) buildShell(c echo.Context, title string, breadcrumb []layout.BreadcrumbItem) (layout.NavData, layout.UserData) {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav("/setting")
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	_ = title
	_ = breadcrumb
	return nav, ud
}

// Index GET /setting/gudang
func (h *GudangHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	items, err := h.svc.List(ctx, true)
	if err != nil {
		slog.ErrorContext(ctx, "list gudang", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "Gudang", nil)
	return RenderHTML(c, http.StatusOK, gudangview.Index(gudangview.IndexProps{
		Nav: nav, User: ud, Items: items, CSRFToken: csrf,
	}))
}

// New GET /setting/gudang/baru
func (h *GudangHandler) New(c echo.Context) error {
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "Tambah Gudang", nil)
	return RenderHTML(c, http.StatusOK, gudangview.Form(gudangview.FormProps{
		Nav: nav, User: ud, Mode: "create", CSRFToken: csrf,
		Form: gudangview.FormData{IsActive: true},
	}))
}

// Create POST /setting/gudang
func (h *GudangHandler) Create(c echo.Context) error {
	in := dto.GudangCreateInput{
		Kode:     c.FormValue("kode"),
		Nama:     c.FormValue("nama"),
		Alamat:   c.FormValue("alamat"),
		Telepon:  c.FormValue("telepon"),
		IsActive: c.FormValue("is_active") == "on" || c.FormValue("is_active") == "true",
	}
	csrf, _ := c.Get("csrf").(string)

	if err := dto.Validate(in); err != nil {
		return h.renderFormError(c, "create", 0, csrf, in, err.Error())
	}

	if _, err := h.svc.Create(c.Request().Context(), in); err != nil {
		return h.renderFormError(c, "create", 0, csrf, in, mapDomainError(err))
	}
	return c.Redirect(http.StatusSeeOther, "/setting/gudang")
}

// Edit GET /setting/gudang/:id/edit
func (h *GudangHandler) Edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	g, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrGudangNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "gudang tidak ditemukan")
		}
		return err
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "Edit Gudang", nil)
	form := gudangview.FormData{
		Kode: g.Kode, Nama: g.Nama, IsActive: g.IsActive,
	}
	if g.Alamat != nil {
		form.Alamat = *g.Alamat
	}
	if g.Telepon != nil {
		form.Telepon = *g.Telepon
	}
	return RenderHTML(c, http.StatusOK, gudangview.Form(gudangview.FormProps{
		Nav: nav, User: ud, Mode: "edit", ID: g.ID, CSRFToken: csrf, Form: form,
	}))
}

// Update POST /setting/gudang/:id
func (h *GudangHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	in := dto.GudangUpdateInput{
		Kode:     c.FormValue("kode"),
		Nama:     c.FormValue("nama"),
		Alamat:   c.FormValue("alamat"),
		Telepon:  c.FormValue("telepon"),
		IsActive: c.FormValue("is_active") == "on" || c.FormValue("is_active") == "true",
	}
	csrf, _ := c.Get("csrf").(string)

	if err := dto.Validate(in); err != nil {
		return h.renderFormErrorUpdate(c, id, csrf, in, err.Error())
	}
	if _, err := h.svc.Update(c.Request().Context(), id, dto.GudangUpdateInput(in)); err != nil {
		if errors.Is(err, domain.ErrGudangNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "gudang tidak ditemukan")
		}
		return h.renderFormErrorUpdate(c, id, csrf, in, mapDomainError(err))
	}
	return c.Redirect(http.StatusSeeOther, "/setting/gudang")
}

// ToggleActive POST /setting/gudang/:id/toggle-active
func (h *GudangHandler) ToggleActive(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	g, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrGudangNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "gudang tidak ditemukan")
		}
		return err
	}
	if err := h.svc.SetActive(c.Request().Context(), id, !g.IsActive); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/setting/gudang")
}

func (h *GudangHandler) renderFormError(c echo.Context, mode string, id int64, csrf string, in dto.GudangCreateInput, msg string) error {
	nav, ud := h.buildShell(c, "Form Gudang", nil)
	return RenderHTML(c, http.StatusUnprocessableEntity, gudangview.Form(gudangview.FormProps{
		Nav: nav, User: ud, Mode: mode, ID: id, CSRFToken: csrf,
		Form: gudangview.FormData{
			Kode: in.Kode, Nama: in.Nama, Alamat: in.Alamat,
			Telepon: in.Telepon, IsActive: in.IsActive,
		},
		Error: msg,
	}))
}

func (h *GudangHandler) renderFormErrorUpdate(c echo.Context, id int64, csrf string, in dto.GudangUpdateInput, msg string) error {
	return h.renderFormError(c, "edit", id, csrf, dto.GudangCreateInput(in), msg)
}

func mapDomainError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

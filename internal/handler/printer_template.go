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
	printerview "github.com/omanjaya/tokobangunan/internal/view/setting/printer"
)

// PrinterTemplateHandler - HTTP handler /setting/printer.
type PrinterTemplateHandler struct {
	svc    *service.PrinterTemplateService
	gudang *service.GudangService
}

func NewPrinterTemplateHandler(s *service.PrinterTemplateService, g *service.GudangService) *PrinterTemplateHandler {
	return &PrinterTemplateHandler{svc: s, gudang: g}
}

func (h *PrinterTemplateHandler) buildShell(c echo.Context) (layout.NavData, layout.UserData) {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav("/setting")
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	return nav, ud
}

// Index GET /setting/printer
func (h *PrinterTemplateHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	items, err := h.svc.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list printer_template", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat template printer")
	}
	gudangs, err := h.gudang.List(ctx, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, printerview.Index(printerview.IndexProps{
		Nav: nav, User: ud, Items: items, Gudangs: gudangs, CSRFToken: csrf,
	}))
}

// New GET /setting/printer/baru
func (h *PrinterTemplateHandler) New(c echo.Context) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudang.List(ctx, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, printerview.Form(printerview.FormProps{
		Nav: nav, User: ud, Mode: "create", CSRFToken: csrf,
		Gudangs: gudangs,
		Form: printerview.FormData{
			Jenis: "kwitansi", LebarChar: 80, PanjangBaris: 33,
			Koordinat: "{}",
		},
	}))
}

// Create POST /setting/printer
func (h *PrinterTemplateHandler) Create(c echo.Context) error {
	in := h.bindForm(c)
	csrf, _ := c.Get("csrf").(string)
	if err := dto.Validate(in); err != nil {
		return h.renderFormErr(c, "create", 0, csrf, in, err.Error())
	}
	if _, err := h.svc.Create(c.Request().Context(), in); err != nil {
		return h.renderFormErr(c, "create", 0, csrf, in, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/setting/printer")
}

// Edit GET /setting/printer/:id/edit
func (h *PrinterTemplateHandler) Edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	ctx := c.Request().Context()
	t, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPrinterTemplateNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "template tidak ditemukan")
		}
		return err
	}
	gudangs, err := h.gudang.List(ctx, true)
	if err != nil {
		return err
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, printerview.Form(printerview.FormProps{
		Nav: nav, User: ud, Mode: "edit", ID: t.ID, CSRFToken: csrf,
		Gudangs: gudangs,
		Form: printerview.FormData{
			GudangID: t.GudangID, Jenis: t.Jenis, Nama: t.Nama,
			LebarChar: t.LebarChar, PanjangBaris: t.PanjangBaris,
			OffsetX: t.OffsetX, OffsetY: t.OffsetY,
			Koordinat: t.Koordinat, IsDefault: t.IsDefault,
		},
	}))
}

// Update POST /setting/printer/:id
func (h *PrinterTemplateHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	in := h.bindForm(c)
	upd := dto.PrinterTemplateUpdateInput(in)
	csrf, _ := c.Get("csrf").(string)
	if err := dto.Validate(upd); err != nil {
		return h.renderFormErr(c, "edit", id, csrf, in, err.Error())
	}
	if _, err := h.svc.Update(c.Request().Context(), id, upd); err != nil {
		if errors.Is(err, domain.ErrPrinterTemplateNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "template tidak ditemukan")
		}
		return h.renderFormErr(c, "edit", id, csrf, in, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/setting/printer")
}

// Delete POST /setting/printer/:id/delete
func (h *PrinterTemplateHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrPrinterTemplateNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "template tidak ditemukan")
		}
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/setting/printer")
}

// Test POST /setting/printer/:id/test - generate ASCII preview.
func (h *PrinterTemplateHandler) Test(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	t, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrPrinterTemplateNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "template tidak ditemukan")
		}
		return err
	}
	preview := h.svc.GeneratePreview(t)
	return c.String(http.StatusOK, preview)
}

func (h *PrinterTemplateHandler) bindForm(c echo.Context) dto.PrinterTemplateCreateInput {
	gudangID, _ := strconv.ParseInt(c.FormValue("gudang_id"), 10, 64)
	lebar, _ := strconv.Atoi(c.FormValue("lebar_char"))
	panjang, _ := strconv.Atoi(c.FormValue("panjang_baris"))
	offsetX, _ := strconv.Atoi(c.FormValue("offset_x"))
	offsetY, _ := strconv.Atoi(c.FormValue("offset_y"))
	return dto.PrinterTemplateCreateInput{
		GudangID:     gudangID,
		Jenis:        c.FormValue("jenis"),
		Nama:         c.FormValue("nama"),
		LebarChar:    lebar,
		PanjangBaris: panjang,
		OffsetX:      offsetX,
		OffsetY:      offsetY,
		Koordinat:    c.FormValue("koordinat"),
		IsDefault:    c.FormValue("is_default") == "on" || c.FormValue("is_default") == "true",
	}
}

func (h *PrinterTemplateHandler) renderFormErr(c echo.Context, mode string, id int64, csrf string, in dto.PrinterTemplateCreateInput, msg string) error {
	ctx := c.Request().Context()
	gudangs, _ := h.gudang.List(ctx, true)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusUnprocessableEntity, printerview.Form(printerview.FormProps{
		Nav: nav, User: ud, Mode: mode, ID: id, CSRFToken: csrf,
		Gudangs: gudangs,
		Form: printerview.FormData{
			GudangID: in.GudangID, Jenis: in.Jenis, Nama: in.Nama,
			LebarChar: in.LebarChar, PanjangBaris: in.PanjangBaris,
			OffsetX: in.OffsetX, OffsetY: in.OffsetY,
			Koordinat: in.Koordinat, IsDefault: in.IsDefault,
		},
		Error: msg,
	}))
}

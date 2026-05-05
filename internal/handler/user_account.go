package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	userview "github.com/omanjaya/tokobangunan/internal/view/setting/user"
)

type UserAccountHandler struct {
	svc        *service.UserAccountService
	gudangRepo *repo.GudangRepo
}

func NewUserAccountHandler(svc *service.UserAccountService, gudangRepo *repo.GudangRepo) *UserAccountHandler {
	return &UserAccountHandler{svc: svc, gudangRepo: gudangRepo}
}

func (h *UserAccountHandler) buildShell(c echo.Context) (layout.NavData, layout.UserData) {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav("/setting")
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	return nav, ud
}

// Index GET /setting/user
func (h *UserAccountHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	q := strings.TrimSpace(c.QueryParam("q"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	roleParam := strings.TrimSpace(c.QueryParam("role"))

	f := repo.ListUserFilter{Query: q, Page: page, PerPage: 25}
	if roleParam != "" {
		f.Role = &roleParam
	}

	res, err := h.svc.List(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, "list user", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat user")
	}

	gudangs, err := h.gudangRepo.List(ctx, true)
	if err != nil {
		slog.ErrorContext(ctx, "list gudang", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}

	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, userview.Index(userview.IndexProps{
		Nav: nav, User: ud,
		Result:    res,
		Query:     q,
		Role:      roleParam,
		Gudangs:   gudangs,
		CSRFToken: csrf,
	}))
}

// New GET /setting/user/baru
func (h *UserAccountHandler) New(c echo.Context) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudangRepo.List(ctx, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, userview.Form(userview.FormProps{
		Nav: nav, User: ud, Mode: "create", CSRFToken: csrf,
		Gudangs: gudangs,
		Form:    userview.FormData{IsActive: true, Role: domain.RoleKasir},
	}))
}

// Create POST /setting/user
func (h *UserAccountHandler) Create(c echo.Context) error {
	in := h.parseCreateForm(c)
	csrf, _ := c.Get("csrf").(string)

	if err := dto.Validate(in); err != nil {
		return h.renderCreateErr(c, csrf, in, err.Error())
	}

	res, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return h.renderCreateErr(c, csrf, in, err.Error())
	}

	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, userview.PasswordCreated(userview.PasswordCreatedProps{
		Nav: nav, User: ud,
		Username:  res.User.Username,
		Password:  res.PlaintextPassword,
		IsReset:   false,
		BackHref:  "/setting/user",
	}))
}

// Edit GET /setting/user/:id/edit
func (h *UserAccountHandler) Edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	u, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserAccountNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user tidak ditemukan")
		}
		return err
	}
	gudangs, err := h.gudangRepo.List(c.Request().Context(), true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat gudang")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c)
	form := userview.FormData{
		Username:    u.Username,
		NamaLengkap: u.NamaLengkap,
		Role:        u.Role,
		GudangID:    u.GudangID,
		IsActive:    u.IsActive,
	}
	if u.Email != nil {
		form.Email = *u.Email
	}
	return RenderHTML(c, http.StatusOK, userview.Form(userview.FormProps{
		Nav: nav, User: ud, Mode: "edit", ID: u.ID, CSRFToken: csrf,
		Gudangs: gudangs, Form: form,
	}))
}

// Update POST /setting/user/:id
func (h *UserAccountHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	in := h.parseUpdateForm(c)
	csrf, _ := c.Get("csrf").(string)

	if err := dto.Validate(in); err != nil {
		return h.renderUpdateErr(c, id, csrf, in, err.Error())
	}
	if _, err := h.svc.Update(c.Request().Context(), id, in); err != nil {
		if errors.Is(err, domain.ErrUserAccountNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user tidak ditemukan")
		}
		return h.renderUpdateErr(c, id, csrf, in, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/setting/user")
}

// ResetPassword POST /setting/user/:id/reset-password
func (h *UserAccountHandler) ResetPassword(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	u, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserAccountNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user tidak ditemukan")
		}
		return err
	}
	plaintext, err := h.svc.ResetPassword(c.Request().Context(), id)
	if err != nil {
		return err
	}
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, userview.PasswordCreated(userview.PasswordCreatedProps{
		Nav: nav, User: ud,
		Username: u.Username, Password: plaintext, IsReset: true,
		BackHref: "/setting/user",
	}))
}

// ToggleActive POST /setting/user/:id/toggle-active
func (h *UserAccountHandler) ToggleActive(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	u, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserAccountNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user tidak ditemukan")
		}
		return err
	}
	// Cegah owner non-aktifkan dirinya sendiri.
	if cur := auth.CurrentUser(c); cur != nil && cur.ID == id && u.IsActive {
		return echo.NewHTTPError(http.StatusBadRequest, "tidak bisa menonaktifkan diri sendiri")
	}
	if err := h.svc.SetActive(c.Request().Context(), id, !u.IsActive); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/setting/user")
}

// ---------- helpers ----------

func (h *UserAccountHandler) parseCreateForm(c echo.Context) dto.UserCreateInput {
	in := dto.UserCreateInput{
		Username:    c.FormValue("username"),
		NamaLengkap: c.FormValue("nama_lengkap"),
		Email:       c.FormValue("email"),
		Role:        c.FormValue("role"),
		GudangID:    parseGudangID(c.FormValue("gudang_id")),
		IsActive:    c.FormValue("is_active") == "on" || c.FormValue("is_active") == "true",
	}
	return in
}

func (h *UserAccountHandler) parseUpdateForm(c echo.Context) dto.UserUpdateInput {
	in := dto.UserUpdateInput{
		Username:    c.FormValue("username"),
		NamaLengkap: c.FormValue("nama_lengkap"),
		Email:       c.FormValue("email"),
		Role:        c.FormValue("role"),
		GudangID:    parseGudangID(c.FormValue("gudang_id")),
		IsActive:    c.FormValue("is_active") == "on" || c.FormValue("is_active") == "true",
	}
	return in
}

func parseGudangID(s string) *int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

func (h *UserAccountHandler) renderCreateErr(c echo.Context, csrf string, in dto.UserCreateInput, msg string) error {
	gudangs, _ := h.gudangRepo.List(c.Request().Context(), true)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusUnprocessableEntity, userview.Form(userview.FormProps{
		Nav: nav, User: ud, Mode: "create", CSRFToken: csrf,
		Gudangs: gudangs,
		Form: userview.FormData{
			Username: in.Username, NamaLengkap: in.NamaLengkap,
			Email: in.Email, Role: in.Role, GudangID: in.GudangID, IsActive: in.IsActive,
		},
		Error: msg,
	}))
}

func (h *UserAccountHandler) renderUpdateErr(c echo.Context, id int64, csrf string, in dto.UserUpdateInput, msg string) error {
	gudangs, _ := h.gudangRepo.List(c.Request().Context(), true)
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusUnprocessableEntity, userview.Form(userview.FormProps{
		Nav: nav, User: ud, Mode: "edit", ID: id, CSRFToken: csrf,
		Gudangs: gudangs,
		Form: userview.FormData{
			Username: in.Username, NamaLengkap: in.NamaLengkap,
			Email: in.Email, Role: in.Role, GudangID: in.GudangID, IsActive: in.IsActive,
		},
		Error: msg,
	}))
}

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
	supplierview "github.com/omanjaya/tokobangunan/internal/view/supplier"
)

// SupplierHandler menangani routes /supplier/*.
type SupplierHandler struct {
	svc *service.SupplierService
}

// NewSupplierHandler konstruktor.
func NewSupplierHandler(svc *service.SupplierService) *SupplierHandler {
	return &SupplierHandler{svc: svc}
}

// Index GET /supplier.
func (h *SupplierHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	status := strings.TrimSpace(c.QueryParam("status"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	filter := repo.ListSupplierFilter{
		Query:   q,
		Page:    page,
		PerPage: 25,
	}
	var isActive *bool
	switch status {
	case "aktif":
		v := true
		isActive = &v
	case "nonaktif":
		v := false
		isActive = &v
	}
	filter.IsActive = isActive

	res, err := h.svc.List(c.Request().Context(), filter)
	if err != nil {
		slog.ErrorContext(c.Request().Context(), "list supplier failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar supplier")
	}

	props := supplierview.IndexProps{
		Nav:        navSupplier(),
		User:       userData(user),
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		Query:      q,
		IsActive:   isActive,
		FlashSuccess: c.QueryParam("flash"),
	}
	if csrf, ok := c.Get("csrf").(string); ok {
		props.CSRFToken = csrf
	}
	return RenderHTML(c, http.StatusOK, supplierview.Index(props))
}

// New GET /supplier/baru.
func (h *SupplierHandler) New(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	props := supplierview.FormProps{
		Nav:    navSupplier(),
		User:   userData(user),
		IsEdit: false,
		Item:   &domain.Supplier{IsActive: true},
	}
	return RenderHTML(c, http.StatusOK, supplierview.Form(props))
}

// Create POST /supplier.
func (h *SupplierHandler) Create(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	var in dto.SupplierCreateInput
	if err := c.Bind(&in); err != nil {
		return h.renderCreateErr(c, user, &in, nil, "Form tidak valid.")
	}
	in.IsActive = c.FormValue("is_active") == "true"

	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderCreateErr(c, user, &in, fe, "")
	}

	s, err := h.svc.Create(c.Request().Context(), service.CreateSupplierInput{
		Kode:     in.Kode,
		Nama:     in.Nama,
		Alamat:   in.Alamat,
		Kontak:   in.Kontak,
		Catatan:  in.Catatan,
		IsActive: in.IsActive,
	})
	if err != nil {
		return h.handleMutationErr(c, user, supplierFromCreateInput(&in), false, 0, err)
	}
	_ = s
	return c.Redirect(http.StatusSeeOther, "/supplier")
}

// Edit GET /supplier/:id/edit.
func (h *SupplierHandler) Edit(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID supplier tidak valid")
	}
	s, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrSupplierTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Supplier tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat supplier")
	}
	props := supplierview.FormProps{
		Nav:    navSupplier(),
		User:   userData(user),
		IsEdit: true,
		Item:   s,
	}
	return RenderHTML(c, http.StatusOK, supplierview.Form(props))
}

// Update POST /supplier/:id.
func (h *SupplierHandler) Update(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID supplier tidak valid")
	}

	var in dto.SupplierUpdateInput
	if err := c.Bind(&in); err != nil {
		return h.renderEditErr(c, user, supplierFromUpdateInput(&in, id), nil, "Form tidak valid.")
	}
	in.ID = id
	in.IsActive = c.FormValue("is_active") == "true"

	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderEditErr(c, user, supplierFromUpdateInput(&in, id), fe, "")
	}

	_, err = h.svc.Update(c.Request().Context(), service.UpdateSupplierInput{
		ID:       id,
		Kode:     in.Kode,
		Nama:     in.Nama,
		Alamat:   in.Alamat,
		Kontak:   in.Kontak,
		Catatan:  in.Catatan,
		IsActive: in.IsActive,
	})
	if err != nil {
		return h.handleMutationErr(c, user, supplierFromUpdateInput(&in, id), true, id, err)
	}
	return c.Redirect(http.StatusSeeOther, "/supplier")
}

// Delete POST /supplier/:id/delete.
func (h *SupplierHandler) Delete(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID supplier tidak valid")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrSupplierTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Supplier tidak ditemukan")
		}
		slog.ErrorContext(c.Request().Context(), "delete supplier failed", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menghapus supplier")
	}
	return c.Redirect(http.StatusSeeOther, "/supplier")
}

// --- helpers ---

func (h *SupplierHandler) renderCreateErr(c echo.Context, user *auth.User, in *dto.SupplierCreateInput, fe map[string]string, general string) error {
	props := supplierview.FormProps{
		Nav:     navSupplier(),
		User:    userData(user),
		IsEdit:  false,
		Item:    supplierFromCreateInput(in),
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, supplierview.Form(props))
}

func (h *SupplierHandler) renderEditErr(c echo.Context, user *auth.User, s *domain.Supplier, fe map[string]string, general string) error {
	props := supplierview.FormProps{
		Nav:     navSupplier(),
		User:    userData(user),
		IsEdit:  true,
		Item:    s,
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, supplierview.Form(props))
}

func (h *SupplierHandler) handleMutationErr(c echo.Context, user *auth.User, s *domain.Supplier, isEdit bool, id int64, err error) error {
	msg := "Gagal menyimpan supplier."
	switch {
	case errors.Is(err, domain.ErrSupplierKodeDuplicate):
		msg = "Kode supplier sudah dipakai."
	case errors.Is(err, domain.ErrSupplierKodeKosong),
		errors.Is(err, domain.ErrSupplierNamaKosong):
		msg = err.Error()
	case errors.Is(err, domain.ErrSupplierTidakDitemukan):
		return echo.NewHTTPError(http.StatusNotFound, "Supplier tidak ditemukan")
	default:
		slog.ErrorContext(c.Request().Context(), "supplier mutation failed", "error", err, "edit", isEdit, "id", id)
	}
	if isEdit {
		return h.renderEditErr(c, user, s, nil, msg)
	}
	return h.renderCreateErr(c, user, supplierCreateInputFromEntity(s), nil, msg)
}

func supplierFromCreateInput(in *dto.SupplierCreateInput) *domain.Supplier {
	if in == nil {
		return &domain.Supplier{IsActive: true}
	}
	s := &domain.Supplier{
		Kode:     in.Kode,
		Nama:     in.Nama,
		IsActive: in.IsActive,
	}
	if in.Alamat != "" {
		v := in.Alamat
		s.Alamat = &v
	}
	if in.Kontak != "" {
		v := in.Kontak
		s.Kontak = &v
	}
	if in.Catatan != "" {
		v := in.Catatan
		s.Catatan = &v
	}
	return s
}

func supplierFromUpdateInput(in *dto.SupplierUpdateInput, id int64) *domain.Supplier {
	if in == nil {
		return &domain.Supplier{ID: id}
	}
	s := &domain.Supplier{
		ID:       id,
		Kode:     in.Kode,
		Nama:     in.Nama,
		IsActive: in.IsActive,
	}
	if in.Alamat != "" {
		v := in.Alamat
		s.Alamat = &v
	}
	if in.Kontak != "" {
		v := in.Kontak
		s.Kontak = &v
	}
	if in.Catatan != "" {
		v := in.Catatan
		s.Catatan = &v
	}
	return s
}

func supplierCreateInputFromEntity(s *domain.Supplier) *dto.SupplierCreateInput {
	if s == nil {
		return &dto.SupplierCreateInput{IsActive: true}
	}
	in := &dto.SupplierCreateInput{
		Kode:     s.Kode,
		Nama:     s.Nama,
		IsActive: s.IsActive,
	}
	if s.Alamat != nil {
		in.Alamat = *s.Alamat
	}
	if s.Kontak != nil {
		in.Kontak = *s.Kontak
	}
	if s.Catatan != nil {
		in.Catatan = *s.Catatan
	}
	return in
}

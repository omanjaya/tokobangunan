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
	mitraview "github.com/omanjaya/tokobangunan/internal/view/mitra"
)

// MitraHandler menangani routes /mitra/*.
type MitraHandler struct {
	svc *service.MitraService
}

// NewMitraHandler konstruktor.
func NewMitraHandler(svc *service.MitraService) *MitraHandler {
	return &MitraHandler{svc: svc}
}

// Index GET /mitra - list mitra dengan filter & pagination.
func (h *MitraHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	tipe := strings.TrimSpace(c.QueryParam("tipe"))
	status := strings.TrimSpace(c.QueryParam("status"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	filter := repo.ListMitraFilter{
		Query:   q,
		Page:    page,
		PerPage: 25,
	}
	if tipe != "" && domain.IsValidMitraTipe(tipe) {
		filter.Tipe = &tipe
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
		slog.ErrorContext(c.Request().Context(), "list mitra failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar mitra")
	}

	props := mitraview.IndexProps{
		Nav:        navMitra(),
		User:       userData(user),
		Items:      res.Items,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		Query:      q,
		Tipe:       tipe,
		IsActive:   isActive,
		FlashSuccess: c.QueryParam("flash"),
	}
	if csrf, ok := c.Get("csrf").(string); ok {
		props.CSRFToken = csrf
	}
	return RenderHTML(c, http.StatusOK, mitraview.Index(props))
}

// New GET /mitra/baru - form create.
func (h *MitraHandler) New(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	props := mitraview.FormProps{
		Nav:    navMitra(),
		User:   userData(user),
		IsEdit: false,
		Item: &domain.Mitra{
			Tipe:           domain.MitraTipeEceran,
			JatuhTempoHari: 30,
			IsActive:       true,
		},
	}
	return RenderHTML(c, http.StatusOK, mitraview.Form(props))
}

// Create POST /mitra.
func (h *MitraHandler) Create(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	var in dto.MitraCreateInput
	if err := c.Bind(&in); err != nil {
		return h.renderCreateErr(c, user, &in, nil, "Form tidak valid.")
	}
	in.IsActive = c.FormValue("is_active") == "true"

	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderCreateErr(c, user, &in, fe, "")
	}

	m, err := h.svc.Create(c.Request().Context(), service.CreateMitraInput{
		Kode:            in.Kode,
		Nama:            in.Nama,
		Alamat:          in.Alamat,
		Kontak:          in.Kontak,
		NPWP:            in.NPWP,
		Tipe:            in.Tipe,
		LimitKreditCent: in.LimitKreditRp * 100,
		JatuhTempoHari:  in.JatuhTempoHari,
		GudangDefaultID: in.GudangDefaultID,
		Catatan:         in.Catatan,
		IsActive:        in.IsActive,
	})
	if err != nil {
		return h.handleMutationErr(c, user, mitraFromCreateInput(&in), false, 0, err)
	}
	return c.Redirect(http.StatusSeeOther, "/mitra/"+strconv.FormatInt(m.ID, 10))
}

// Show GET /mitra/:id.
func (h *MitraHandler) Show(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	m, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMitraTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat mitra")
	}
	tab := c.QueryParam("tab")
	if tab == "" {
		tab = "info"
	}
	props := mitraview.ShowProps{
		Nav:    navMitra(),
		User:   userData(user),
		Item:   m,
		Active: tab,
	}
	return RenderHTML(c, http.StatusOK, mitraview.Show(props))
}

// Edit GET /mitra/:id/edit.
func (h *MitraHandler) Edit(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	m, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMitraTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat mitra")
	}
	props := mitraview.FormProps{
		Nav:    navMitra(),
		User:   userData(user),
		IsEdit: true,
		Item:   m,
	}
	return RenderHTML(c, http.StatusOK, mitraview.Form(props))
}

// Update POST /mitra/:id.
func (h *MitraHandler) Update(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}

	var in dto.MitraUpdateInput
	if err := c.Bind(&in); err != nil {
		return h.handleMutationErr(c, user, mitraEditInputFromUpdate(&in, id), true, id, errors.New("form tidak valid"))
	}
	in.ID = id
	in.IsActive = c.FormValue("is_active") == "true"

	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderEditErr(c, user, mitraEditInputFromUpdate(&in, id), fe, "")
	}

	_, err = h.svc.Update(c.Request().Context(), service.UpdateMitraInput{
		ID:              id,
		Kode:            in.Kode,
		Nama:            in.Nama,
		Alamat:          in.Alamat,
		Kontak:          in.Kontak,
		NPWP:            in.NPWP,
		Tipe:            in.Tipe,
		LimitKreditCent: in.LimitKreditRp * 100,
		JatuhTempoHari:  in.JatuhTempoHari,
		GudangDefaultID: in.GudangDefaultID,
		Catatan:         in.Catatan,
		IsActive:        in.IsActive,
		Version:         in.Version,
	})
	if err != nil {
		return h.handleMutationErr(c, user, mitraEditInputFromUpdate(&in, id), true, id, err)
	}
	return c.Redirect(http.StatusSeeOther, "/mitra/"+strconv.FormatInt(id, 10))
}

// Delete POST /mitra/:id/delete.
func (h *MitraHandler) Delete(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID mitra tidak valid")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrMitraTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
		}
		slog.ErrorContext(c.Request().Context(), "delete mitra failed", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menghapus mitra")
	}
	return c.Redirect(http.StatusSeeOther, "/mitra")
}

// SearchAjax GET /mitra/search?q=...
func (h *MitraHandler) SearchAjax(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 10
	}
	items, err := h.svc.Search(c.Request().Context(), q, limit)
	if err != nil {
		slog.ErrorContext(c.Request().Context(), "search mitra failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mencari mitra")
	}
	return RenderHTML(c, http.StatusOK, mitraview.SearchResults(mitraview.SearchResultsProps{
		Items: items,
		Query: q,
	}))
}

// --- helpers ---

func (h *MitraHandler) renderCreateErr(c echo.Context, user *auth.User, in *dto.MitraCreateInput, fe map[string]string, general string) error {
	props := mitraview.FormProps{
		Nav:     navMitra(),
		User:    userData(user),
		IsEdit:  false,
		Item:    mitraFromCreateInput(in),
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, mitraview.Form(props))
}

func (h *MitraHandler) renderEditErr(c echo.Context, user *auth.User, m *domain.Mitra, fe map[string]string, general string) error {
	props := mitraview.FormProps{
		Nav:     navMitra(),
		User:    userData(user),
		IsEdit:  true,
		Item:    m,
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, mitraview.Form(props))
}

// handleMutationErr translate domain/repo error → form rerender atau response.
func (h *MitraHandler) handleMutationErr(c echo.Context, user *auth.User, m *domain.Mitra, isEdit bool, id int64, err error) error {
	msg := "Gagal menyimpan mitra."
	status := http.StatusUnprocessableEntity
	switch {
	case errors.Is(err, domain.ErrConflict):
		msg = "Data mitra sudah diubah pengguna lain. Silakan refresh dan coba lagi."
		status = http.StatusConflict
	case errors.Is(err, domain.ErrMitraKodeDuplicate):
		msg = "Kode mitra sudah dipakai."
	case errors.Is(err, domain.ErrMitraKodeKosong),
		errors.Is(err, domain.ErrMitraNamaKosong),
		errors.Is(err, domain.ErrMitraTipeInvalid),
		errors.Is(err, domain.ErrMitraLimitNegatif),
		errors.Is(err, domain.ErrMitraTempoNegatif):
		msg = err.Error()
	case errors.Is(err, domain.ErrMitraTidakDitemukan):
		return echo.NewHTTPError(http.StatusNotFound, "Mitra tidak ditemukan")
	default:
		slog.ErrorContext(c.Request().Context(), "mitra mutation failed", "error", err, "edit", isEdit, "id", id)
	}
	if isEdit {
		return h.renderEditErrStatus(c, user, m, nil, msg, status)
	}
	return h.renderCreateErr(c, user, createInputFromMitra(m), nil, msg)
}

// renderEditErrStatus - varian renderEditErr dengan custom HTTP status.
func (h *MitraHandler) renderEditErrStatus(c echo.Context, user *auth.User, m *domain.Mitra, fe map[string]string, general string, status int) error {
	props := mitraview.FormProps{
		Nav:     navMitra(),
		User:    userData(user),
		IsEdit:  true,
		Item:    m,
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, status, mitraview.Form(props))
}

func mitraFromCreateInput(in *dto.MitraCreateInput) *domain.Mitra {
	if in == nil {
		return &domain.Mitra{Tipe: domain.MitraTipeEceran, IsActive: true}
	}
	m := &domain.Mitra{
		Kode:           in.Kode,
		Nama:           in.Nama,
		Tipe:           in.Tipe,
		LimitKredit:    in.LimitKreditRp * 100,
		JatuhTempoHari: in.JatuhTempoHari,
		IsActive:       in.IsActive,
	}
	if in.Alamat != "" {
		v := in.Alamat
		m.Alamat = &v
	}
	if in.Kontak != "" {
		v := in.Kontak
		m.Kontak = &v
	}
	if in.NPWP != "" {
		v := in.NPWP
		m.NPWP = &v
	}
	if in.Catatan != "" {
		v := in.Catatan
		m.Catatan = &v
	}
	if in.GudangDefaultID > 0 {
		v := in.GudangDefaultID
		m.GudangDefaultID = &v
	}
	return m
}

func createInputFromMitra(m *domain.Mitra) *dto.MitraCreateInput {
	in := &dto.MitraCreateInput{
		Kode:           m.Kode,
		Nama:           m.Nama,
		Tipe:           m.Tipe,
		LimitKreditRp:  m.LimitKredit / 100,
		JatuhTempoHari: m.JatuhTempoHari,
		IsActive:       m.IsActive,
	}
	if m.Alamat != nil {
		in.Alamat = *m.Alamat
	}
	if m.Kontak != nil {
		in.Kontak = *m.Kontak
	}
	if m.NPWP != nil {
		in.NPWP = *m.NPWP
	}
	if m.Catatan != nil {
		in.Catatan = *m.Catatan
	}
	if m.GudangDefaultID != nil {
		in.GudangDefaultID = *m.GudangDefaultID
	}
	return in
}

func mitraEditInputFromUpdate(in *dto.MitraUpdateInput, id int64) *domain.Mitra {
	if in == nil {
		return &domain.Mitra{ID: id, Tipe: domain.MitraTipeEceran}
	}
	m := &domain.Mitra{
		ID:             id,
		Kode:           in.Kode,
		Nama:           in.Nama,
		Tipe:           in.Tipe,
		LimitKredit:    in.LimitKreditRp * 100,
		JatuhTempoHari: in.JatuhTempoHari,
		IsActive:       in.IsActive,
		Version:        in.Version,
	}
	if in.Alamat != "" {
		v := in.Alamat
		m.Alamat = &v
	}
	if in.Kontak != "" {
		v := in.Kontak
		m.Kontak = &v
	}
	if in.NPWP != "" {
		v := in.NPWP
		m.NPWP = &v
	}
	if in.Catatan != "" {
		v := in.Catatan
		m.Catatan = &v
	}
	if in.GudangDefaultID > 0 {
		v := in.GudangDefaultID
		m.GudangDefaultID = &v
	}
	return m
}

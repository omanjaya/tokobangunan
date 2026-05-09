package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/service"
	diskonview "github.com/omanjaya/tokobangunan/internal/view/diskon"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// DiskonMasterHandler - HTTP handler master diskon.
type DiskonMasterHandler struct {
	svc *service.DiskonMasterService
}

func NewDiskonMasterHandler(svc *service.DiskonMasterService) *DiskonMasterHandler {
	return &DiskonMasterHandler{svc: svc}
}

func (h *DiskonMasterHandler) shell(c echo.Context) (layout.NavData, layout.UserData) {
	return layout.DefaultNav("/setting"), userData(auth.CurrentUser(c))
}

// Index GET /setting/diskon
func (h *DiskonMasterHandler) Index(c echo.Context) error {
	items, err := h.svc.List(c.Request().Context(), false)
	if err != nil {
		return err
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.shell(c)
	return RenderHTML(c, http.StatusOK, diskonview.Index(diskonview.IndexProps{
		Nav: nav, User: ud, Items: items, CSRFToken: csrf,
	}))
}

// New GET /setting/diskon/baru
func (h *DiskonMasterHandler) New(c echo.Context) error {
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.shell(c)
	return RenderHTML(c, http.StatusOK, diskonview.Form(diskonview.FormProps{
		Nav: nav, User: ud, Mode: "create", CSRFToken: csrf,
		Form: dto.DiskonMasterInput{
			Tipe:        "persen",
			IsActive:    true,
			BerlakuDari: time.Now().Format("2006-01-02"),
		},
	}))
}

// Create POST /setting/diskon
func (h *DiskonMasterHandler) Create(c echo.Context) error {
	in, err := bindDiskonInput(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if _, err := h.svc.Create(c.Request().Context(), in); err != nil {
		return h.renderFormError(c, "create", 0, in, err)
	}
	return c.Redirect(http.StatusSeeOther, "/setting/diskon")
}

// Edit GET /setting/diskon/:id/edit
func (h *DiskonMasterHandler) Edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	d, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDiskonNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "diskon tidak ditemukan")
		}
		return err
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.shell(c)
	form := dto.DiskonMasterInput{
		Kode:        d.Kode,
		Nama:        d.Nama,
		Tipe:        d.Tipe,
		Nilai:       d.Nilai,
		MinSubtotal: d.MinSubtotal / 100,
		BerlakuDari: d.BerlakuDari.Format("2006-01-02"),
		IsActive:    d.IsActive,
	}
	if d.MaxDiskon != nil {
		form.MaxDiskon = *d.MaxDiskon / 100
	}
	if d.BerlakuSampai != nil {
		form.BerlakuSampai = d.BerlakuSampai.Format("2006-01-02")
	}
	return RenderHTML(c, http.StatusOK, diskonview.Form(diskonview.FormProps{
		Nav: nav, User: ud, Mode: "edit", ID: d.ID, CSRFToken: csrf, Form: form,
	}))
}

// Update POST /setting/diskon/:id
func (h *DiskonMasterHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	in, err := bindDiskonInput(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if _, err := h.svc.Update(c.Request().Context(), id, in); err != nil {
		if errors.Is(err, domain.ErrDiskonNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "diskon tidak ditemukan")
		}
		return h.renderFormError(c, "edit", id, in, err)
	}
	return c.Redirect(http.StatusSeeOther, "/setting/diskon")
}

// Toggle POST /setting/diskon/:id/toggle
func (h *DiskonMasterHandler) Toggle(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	d, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDiskonNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "diskon tidak ditemukan")
		}
		return err
	}
	if err := h.svc.Toggle(c.Request().Context(), id, !d.IsActive); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/setting/diskon")
}

// Delete POST /setting/diskon/:id/delete (soft delete via is_active=false).
func (h *DiskonMasterHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrDiskonNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "diskon tidak ditemukan")
		}
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/setting/diskon")
}

// Applicable GET /penjualan/diskon-applicable?subtotal=<cents>
// Return JSON list of applicable diskon dengan computed amount (cents).
func (h *DiskonMasterHandler) Applicable(c echo.Context) error {
	subStr := c.QueryParam("subtotal")
	subtotal, err := strconv.ParseInt(subStr, 10, 64)
	if err != nil || subtotal < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "parameter subtotal tidak valid")
	}
	now := time.Now()
	items, err := h.svc.ListApplicable(c.Request().Context(), subtotal, now)
	if err != nil {
		return err
	}
	type itemJSON struct {
		ID     int64  `json:"id"`
		Kode   string `json:"kode"`
		Nama   string `json:"nama"`
		Tipe   string `json:"tipe"`
		Nilai  float64 `json:"nilai"`
		Amount int64  `json:"amount"`
	}
	out := make([]itemJSON, 0, len(items))
	for _, d := range items {
		out = append(out, itemJSON{
			ID: d.ID, Kode: d.Kode, Nama: d.Nama,
			Tipe: d.Tipe, Nilai: d.Nilai,
			Amount: d.Apply(subtotal),
		})
	}
	return c.JSON(http.StatusOK, out)
}

func bindDiskonInput(c echo.Context) (dto.DiskonMasterInput, error) {
	in := dto.DiskonMasterInput{
		Kode:          c.FormValue("kode"),
		Nama:          c.FormValue("nama"),
		Tipe:          c.FormValue("tipe"),
		BerlakuDari:   c.FormValue("berlaku_dari"),
		BerlakuSampai: c.FormValue("berlaku_sampai"),
		IsActive:      c.FormValue("is_active") == "on" || c.FormValue("is_active") == "true",
	}
	if v := c.FormValue("nilai"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return in, errors.New("nilai tidak valid")
		}
		in.Nilai = f
	}
	if v := c.FormValue("min_subtotal"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return in, errors.New("min_subtotal tidak valid")
		}
		in.MinSubtotal = n
	}
	if v := c.FormValue("max_diskon"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return in, errors.New("max_diskon tidak valid")
		}
		in.MaxDiskon = n
	}
	return in, nil
}

func (h *DiskonMasterHandler) renderFormError(c echo.Context, mode string, id int64, in dto.DiskonMasterInput, err error) error {
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.shell(c)
	props := diskonview.FormProps{
		Nav: nav, User: ud, Mode: mode, ID: id, CSRFToken: csrf, Form: in,
	}
	var fes dto.FieldErrors
	if errors.As(err, &fes) {
		props.Errors = fes
	} else {
		props.General = humanizeDiskonError(err)
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, diskonview.Form(props))
}

func humanizeDiskonError(err error) string {
	switch {
	case errors.Is(err, domain.ErrDiskonKodeDuplikat):
		return "Kode diskon sudah dipakai."
	case errors.Is(err, domain.ErrDiskonTanggalInvalid):
		return "Tanggal berlaku tidak valid."
	case err == nil:
		return ""
	default:
		return "Gagal menyimpan diskon: " + err.Error()
	}
}

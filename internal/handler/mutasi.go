package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	mutasiview "github.com/omanjaya/tokobangunan/internal/view/mutasi"
)

// MutasiHandler - HTTP handler modul mutasi antar gudang.
type MutasiHandler struct {
	mutasi *service.MutasiService
	gudang *service.GudangService
	produk *service.ProdukService
	satuan *service.SatuanService
}

func NewMutasiHandler(m *service.MutasiService, g *service.GudangService,
	p *service.ProdukService, s *service.SatuanService) *MutasiHandler {
	return &MutasiHandler{mutasi: m, gudang: g, produk: p, satuan: s}
}

// Index GET /mutasi
func (h *MutasiHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	filter := repo.ListMutasiFilter{Page: page, PerPage: 25}

	if v := c.QueryParam("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.From = &t
		}
	}
	if v := c.QueryParam("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.To = &t
		}
	}
	if v, _ := strconv.ParseInt(c.QueryParam("gudang_asal_id"), 10, 64); v > 0 {
		filter.GudangAsalID = &v
	}
	if v, _ := strconv.ParseInt(c.QueryParam("gudang_tujuan_id"), 10, 64); v > 0 {
		filter.GudangTujuanID = &v
	}
	if v := strings.TrimSpace(c.QueryParam("status")); v != "" {
		filter.Status = &v
	}

	// Scope: non-owner/admin hanya lihat mutasi yg melibatkan gudangnya
	// (asal ATAU tujuan). Override input user.
	if u := auth.CurrentUser(c); u != nil && !isPrivilegedRole(u.Role) {
		filter.GudangAsalID = nil
		filter.GudangTujuanID = nil
		if u.GudangID != nil {
			gid := *u.GudangID
			filter.UserScopeGudangID = &gid
		} else {
			impossible := int64(1 << 62)
			filter.UserScopeGudangID = &impossible
		}
	}

	res, err := h.mutasi.List(ctx, filter)
	if err != nil {
		return err
	}

	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}
	gudangIdx := map[int64]string{}
	for _, g := range gudangs {
		gudangIdx[g.ID] = g.Nama
	}

	rows := make([]mutasiview.Row, 0, len(res.Items))
	for _, m := range res.Items {
		rows = append(rows, mutasiview.Row{
			Mutasi:           m,
			GudangAsalNama:   gudangIdx[m.GudangAsalID],
			GudangTujuanNama: gudangIdx[m.GudangTujuanID],
			JumlahItem:       len(m.Items),
		})
	}

	props := mutasiview.IndexProps{
		Nav:        layout.DefaultNav("/mutasi"),
		User:       userData(auth.CurrentUser(c)),
		Rows:       rows,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		From:       c.QueryParam("from"),
		To:         c.QueryParam("to"),
		Status:     c.QueryParam("status"),
		Gudangs:    gudangs,
	}
	if filter.GudangAsalID != nil {
		props.GudangAsalID = *filter.GudangAsalID
	}
	if filter.GudangTujuanID != nil {
		props.GudangTujuanID = *filter.GudangTujuanID
	}
	return RenderHTML(c, http.StatusOK, mutasiview.Index(props))
}

// New GET /mutasi/baru
func (h *MutasiHandler) New(c echo.Context) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}
	satuans, err := h.satuanLite(ctx)
	if err != nil {
		return err
	}

	user := auth.CurrentUser(c)
	in := dto.MutasiCreateInput{}
	if user != nil && user.GudangID != nil {
		in.GudangAsalID = *user.GudangID
	}

	props := mutasiview.FormProps{
		Nav:         layout.DefaultNav("/mutasi"),
		User:        userData(auth.CurrentUser(c)),
		Input:       in,
		Gudangs:     gudangs,
		Satuans:     satuans,
		DefaultDate: time.Now().Format("2006-01-02"),
		ClientUUID:  uuid.New().String(),
	}
	return RenderHTML(c, http.StatusOK, mutasiview.Form(props))
}

// Create POST /mutasi
func (h *MutasiHandler) Create(c echo.Context) error {
	in, err := bindMutasiInput(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	user := auth.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
	}

	created, err := h.mutasi.Create(c.Request().Context(), in, user.ID)
	if err != nil {
		return h.renderFormError(c, in, err)
	}

	target := fmt.Sprintf("/mutasi/%d", created.ID)
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", target)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// Show GET /mutasi/:id
func (h *MutasiHandler) Show(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	return h.renderShow(c, id, "")
}

// Submit POST /mutasi/:id/submit
func (h *MutasiHandler) Submit(c echo.Context) error {
	return h.transition(c, func(ctx context.Context, id, uid int64) error {
		return h.mutasi.Submit(ctx, id, uid)
	})
}

// Receive POST /mutasi/:id/receive
func (h *MutasiHandler) Receive(c echo.Context) error {
	return h.transition(c, func(ctx context.Context, id, uid int64) error {
		return h.mutasi.Receive(ctx, id, uid)
	})
}

// Cancel POST /mutasi/:id/cancel
func (h *MutasiHandler) Cancel(c echo.Context) error {
	return h.transition(c, func(ctx context.Context, id, uid int64) error {
		return h.mutasi.Cancel(ctx, id, uid)
	})
}

func (h *MutasiHandler) transition(c echo.Context, fn func(context.Context, int64, int64) error) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
	}
	// Scope: user hanya boleh transisi mutasi yg melibatkan gudangnya.
	existing, gerr := h.mutasi.Get(c.Request().Context(), id)
	if gerr != nil {
		if errors.Is(gerr, domain.ErrMutasiNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "mutasi tidak ditemukan")
		}
		return gerr
	}
	if err := enforceGudangScopeAny(c, existing.GudangAsalID, existing.GudangTujuanID); err != nil {
		return err
	}
	if err := fn(c.Request().Context(), id, user.ID); err != nil {
		return h.renderShow(c, id, humanizeMutasiError(err))
	}
	target := fmt.Sprintf("/mutasi/%d", id)
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", target)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

func (h *MutasiHandler) renderShow(c echo.Context, id int64, flashErr string) error {
	ctx := c.Request().Context()
	m, err := h.mutasi.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrMutasiNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "mutasi tidak ditemukan")
		}
		return err
	}
	if err := enforceGudangScopeAny(c, m.GudangAsalID, m.GudangTujuanID); err != nil {
		return err
	}
	gAsal, _ := h.gudang.Get(ctx, m.GudangAsalID)
	gTujuan, _ := h.gudang.Get(ctx, m.GudangTujuanID)
	asalNama := ""
	tujuanNama := ""
	if gAsal != nil {
		asalNama = gAsal.Nama
	}
	if gTujuan != nil {
		tujuanNama = gTujuan.Nama
	}

	user := auth.CurrentUser(c)
	canReceive := m.Status == domain.StatusDikirim
	if canReceive && user != nil && user.Role != "owner" && user.Role != "admin" &&
		user.GudangID != nil && *user.GudangID != m.GudangTujuanID {
		canReceive = false
	}

	items := make([]mutasiview.ShowItem, 0, len(m.Items))
	for _, it := range m.Items {
		text := "—"
		if it.HargaInternal != nil {
			text = formatRupiahCents(*it.HargaInternal)
		}
		items = append(items, mutasiview.ShowItem{Item: it, HargaInternalText: text})
	}

	props := mutasiview.ShowProps{
		Nav:              layout.DefaultNav("/mutasi"),
		User:             userData(auth.CurrentUser(c)),
		Mutasi:           *m,
		GudangAsalNama:   asalNama,
		GudangTujuanNama: tujuanNama,
		Items:            items,
		CanSubmit:        m.Status == domain.StatusDraft,
		CanReceive:       canReceive,
		CanCancel:        m.Status == domain.StatusDraft,
		FlashError:       flashErr,
	}
	status := http.StatusOK
	if flashErr != "" {
		status = http.StatusUnprocessableEntity
	}
	return RenderHTML(c, status, mutasiview.Show(props))
}

func (h *MutasiHandler) renderFormError(c echo.Context, in dto.MutasiCreateInput, opErr error) error {
	ctx := c.Request().Context()
	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}
	satuans, err := h.satuanLite(ctx)
	if err != nil {
		return err
	}
	props := mutasiview.FormProps{
		Nav:         layout.DefaultNav("/mutasi"),
		User:        userData(auth.CurrentUser(c)),
		Input:       in,
		Gudangs:     gudangs,
		Satuans:     satuans,
		DefaultDate: time.Now().Format("2006-01-02"),
		ClientUUID:  emptyOr(in.ClientUUID, uuid.New().String()),
	}
	if fes, ok := dto.CollectFieldErrors(opErr); ok {
		props.Errors = fes
	} else {
		props.General = humanizeMutasiError(opErr)
	}
	status := http.StatusUnprocessableEntity
	if c.Request().Header.Get("HX-Request") == "true" {
		return RenderHTML(c, status, mutasiview.FormCard(props))
	}
	return RenderHTML(c, status, mutasiview.Form(props))
}

// ----- helpers ---------------------------------------------------------------

func (h *MutasiHandler) gudangLite(ctx context.Context) ([]mutasiview.GudangLite, error) {
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]mutasiview.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, mutasiview.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

func (h *MutasiHandler) satuanLite(ctx context.Context) ([]mutasiview.SatuanLite, error) {
	list, err := h.satuan.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]mutasiview.SatuanLite, 0, len(list))
	for _, s := range list {
		out = append(out, mutasiview.SatuanLite{ID: s.ID, Kode: s.Kode, Nama: s.Nama})
	}
	return out, nil
}

// bindMutasiInput - parse multipart/form-urlencoded ke MutasiCreateInput.
// Format item: items[N][produk_id], items[N][qty], items[N][satuan_id], dst.
func bindMutasiInput(c echo.Context) (dto.MutasiCreateInput, error) {
	in := dto.MutasiCreateInput{
		Tanggal:    c.FormValue("tanggal"),
		Catatan:    c.FormValue("catatan"),
		ClientUUID: c.FormValue("client_uuid"),
	}
	if v, err := strconv.ParseInt(c.FormValue("gudang_asal_id"), 10, 64); err == nil {
		in.GudangAsalID = v
	}
	if v, err := strconv.ParseInt(c.FormValue("gudang_tujuan_id"), 10, 64); err == nil {
		in.GudangTujuanID = v
	}
	in.SubmitNow = c.FormValue("submit_now") == "true"

	form, err := c.FormParams()
	if err != nil {
		return in, err
	}
	for i := 0; i < 200; i++ {
		pid := form.Get(fmt.Sprintf("items[%d][produk_id]", i))
		if pid == "" {
			continue
		}
		produkID, _ := strconv.ParseInt(pid, 10, 64)
		if produkID <= 0 {
			continue
		}
		qty, _ := strconv.ParseFloat(form.Get(fmt.Sprintf("items[%d][qty]", i)), 64)
		satuanID, _ := strconv.ParseInt(form.Get(fmt.Sprintf("items[%d][satuan_id]", i)), 10, 64)
		item := dto.MutasiItemInput{
			ProdukID: produkID,
			Qty:      qty,
			SatuanID: satuanID,
			Catatan:  form.Get(fmt.Sprintf("items[%d][catatan]", i)),
		}
		if v := form.Get(fmt.Sprintf("items[%d][harga_internal]", i)); v != "" {
			if cents, err := strconv.ParseInt(v, 10, 64); err == nil {
				item.HargaInternal = &cents
			}
		}
		in.Items = append(in.Items, item)
	}
	return in, nil
}

func humanizeMutasiError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, domain.ErrMutasiKosong):
		return "Item mutasi tidak boleh kosong."
	case errors.Is(err, domain.ErrAsalSamaTujuan):
		return "Gudang asal dan tujuan tidak boleh sama."
	case errors.Is(err, domain.ErrTransisiInvalid):
		return "Transisi status tidak diperbolehkan."
	case errors.Is(err, domain.ErrStokTidakCukup):
		return err.Error()
	case errors.Is(err, domain.ErrMutasiNotFound):
		return "Mutasi tidak ditemukan."
	default:
		return "Gagal memproses mutasi: " + err.Error()
	}
}

func formatRupiahCents(cents int64) string {
	rupiah := cents / 100
	return fmt.Sprintf("Rp %d", rupiah)
}

func emptyOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

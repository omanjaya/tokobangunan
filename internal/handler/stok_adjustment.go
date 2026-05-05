package handler

import (
	"context"
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
	stokview "github.com/omanjaya/tokobangunan/internal/view/stok"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// StokAdjustmentHandler menangani routes /stok/adjust*.
type StokAdjustmentHandler struct {
	svc    *service.StokAdjustmentService
	gudang *service.GudangService
	produk *service.ProdukService
	satuan *service.SatuanService
}

// NewStokAdjustmentHandler konstruktor.
func NewStokAdjustmentHandler(
	svc *service.StokAdjustmentService,
	gudang *service.GudangService,
	produk *service.ProdukService,
	satuan *service.SatuanService,
) *StokAdjustmentHandler {
	return &StokAdjustmentHandler{svc: svc, gudang: gudang, produk: produk, satuan: satuan}
}

// New GET /stok/adjust — render form.
func (h *StokAdjustmentHandler) New(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}

	in := dto.StokAdjustmentInput{}
	// Pre-fill gudang dari user atau pertama tersedia.
	if user.GudangID != nil {
		in.GudangID = *user.GudangID
	} else if len(gudangs) > 0 {
		in.GudangID = gudangs[0].ID
	}

	props := stokview.AdjustFormProps{
		Nav:     layout.DefaultNav("/stok"),
		User:    userData(user),
		Gudangs: gudangs,
		Input:   in,
	}
	return RenderHTML(c, http.StatusOK, stokview.Adjust(props))
}

// Create POST /stok/adjust — bind, validate, panggil service.
func (h *StokAdjustmentHandler) Create(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	in := dto.StokAdjustmentInput{}
	if err := c.Bind(&in); err != nil {
		return h.renderFormErr(c, user, in, nil, "Form tidak valid.")
	}
	if err := in.Validate(); err != nil {
		// Map sentinel error → field error untuk highlight di UI.
		fe := map[string]string{}
		switch err {
		case domain.ErrAdjGudangWajib:
			fe["gudangid"] = "Gudang wajib dipilih"
		case domain.ErrAdjProdukWajib:
			fe["produkid"] = "Produk wajib dipilih"
		case domain.ErrAdjSatuanWajib:
			fe["satuanid"] = "Satuan wajib dipilih"
		case domain.ErrAdjQtyNol:
			fe["qty"] = "Qty tidak boleh 0"
		case domain.ErrAdjKategoriInvalid:
			fe["kategori"] = "Kategori tidak valid"
		}
		return h.renderFormErr(c, user, in, fe, err.Error())
	}

	if _, err := h.svc.Create(ctx, user.ID, in); err != nil {
		slog.ErrorContext(ctx, "create stok adjustment failed", "error", err)
		return h.renderFormErr(c, user, in, nil, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/stok/adjust/history")
}

// History GET /stok/adjust/history — list dengan filter.
func (h *StokAdjustmentHandler) History(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	gudangID, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	kategori := strings.TrimSpace(c.QueryParam("kategori"))
	from := strings.TrimSpace(c.QueryParam("from"))
	to := strings.TrimSpace(c.QueryParam("to"))

	f := repo.ListAdjFilter{
		Kategori: kategori,
		Page:     page,
		PerPage:  25,
	}
	if gudangID > 0 {
		gid := gudangID
		f.GudangID = &gid
	}
	if from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.From = &t
		}
	}
	if to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			// To = end of day inclusive.
			t = t.Add(24*time.Hour - time.Second)
			f.To = &t
		}
	}

	res, err := h.svc.List(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, "list stok adjustment failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat riwayat penyesuaian")
	}

	gudangs, err := h.gudangLite(ctx)
	if err != nil {
		return err
	}

	rows := make([]stokview.AdjustHistoryRow, 0, len(res.Items))
	for _, a := range res.Items {
		rows = append(rows, h.toHistoryRow(ctx, a))
	}

	props := stokview.AdjustHistoryProps{
		Nav:        layout.DefaultNav("/stok"),
		User:       userData(user),
		Gudangs:    gudangs,
		Items:      rows,
		Total:      res.Total,
		Page:       res.Page,
		PerPage:    res.PerPage,
		TotalPages: res.TotalPages,
		GudangID:   gudangID,
		Kategori:   kategori,
		From:       from,
		To:         to,
	}
	return RenderHTML(c, http.StatusOK, stokview.AdjustHistory(props))
}

// --- helpers ---

func (h *StokAdjustmentHandler) gudangLite(ctx context.Context) ([]stokview.GudangLite, error) {
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]stokview.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, stokview.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

func (h *StokAdjustmentHandler) renderFormErr(
	c echo.Context, user *auth.User, in dto.StokAdjustmentInput, fe map[string]string, general string,
) error {
	gudangs, _ := h.gudangLite(c.Request().Context())
	props := stokview.AdjustFormProps{
		Nav:     layout.DefaultNav("/stok"),
		User:    userData(user),
		Gudangs: gudangs,
		Input:   in,
		Errors:  fe,
		General: general,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, stokview.Adjust(props))
}

// toHistoryRow memetakan domain → row view dengan resolve nama gudang/produk/satuan.
// Best-effort: kalau lookup gagal, tampilkan ID supaya history tetap render.
func (h *StokAdjustmentHandler) toHistoryRow(ctx context.Context, a domain.StokAdjustment) stokview.AdjustHistoryRow {
	r := stokview.AdjustHistoryRow{
		ID:       a.ID,
		Tanggal:  a.CreatedAt,
		Qty:      a.Qty,
		Kategori: a.Kategori,
	}
	if a.Catatan != nil {
		r.Catatan = *a.Catatan
	}
	if g, err := h.gudang.Get(ctx, a.GudangID); err == nil {
		r.GudangNama = g.Nama
	} else {
		r.GudangNama = "Gudang #" + strconv.FormatInt(a.GudangID, 10)
	}
	if p, err := h.produk.Get(ctx, a.ProdukID); err == nil {
		r.ProdukNama = p.Nama
		r.ProdukSKU = p.SKU
	} else {
		r.ProdukNama = "Produk #" + strconv.FormatInt(a.ProdukID, 10)
	}
	if s, err := h.satuan.Get(ctx, a.SatuanID); err == nil {
		r.SatuanKode = s.Kode
	}
	if a.UserNama != "" {
		r.UserNama = a.UserNama
	} else {
		r.UserNama = "User #" + strconv.FormatInt(a.UserID, 10)
	}
	return r
}

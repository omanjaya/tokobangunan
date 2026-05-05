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
	pembelianview "github.com/omanjaya/tokobangunan/internal/view/pembelian"
)

// PembelianHandler menangani routes /pembelian/*.
type PembelianHandler struct {
	svc          *service.PembelianService
	supplierSvc  *service.SupplierService
	produkSvc    *service.ProdukService
	gudangRepo   *repo.GudangRepo
	satuanRepo   *repo.SatuanRepo
	supplierRepo *repo.SupplierRepo
}

// NewPembelianHandler konstruktor.
func NewPembelianHandler(
	svc *service.PembelianService,
	supplierSvc *service.SupplierService,
	produkSvc *service.ProdukService,
	gudangRepo *repo.GudangRepo,
	satuanRepo *repo.SatuanRepo,
	supplierRepo *repo.SupplierRepo,
) *PembelianHandler {
	return &PembelianHandler{
		svc:          svc,
		supplierSvc:  supplierSvc,
		produkSvc:    produkSvc,
		gudangRepo:   gudangRepo,
		satuanRepo:   satuanRepo,
		supplierRepo: supplierRepo,
	}
}

// Index GET /pembelian.
func (h *PembelianHandler) Index(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	supplierID, _ := strconv.ParseInt(c.QueryParam("supplier_id"), 10, 64)
	gudangID, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	status := strings.TrimSpace(c.QueryParam("status"))
	from := strings.TrimSpace(c.QueryParam("from"))
	to := strings.TrimSpace(c.QueryParam("to"))

	f := repo.ListPembelianFilter{
		SupplierID:  supplierID,
		GudangID:    gudangID,
		StatusBayar: status,
		Page:        page,
		PerPage:     25,
	}
	if t, err := time.Parse("2006-01-02", from); err == nil {
		f.DariTanggal = &t
	}
	if t, err := time.Parse("2006-01-02", to); err == nil {
		f.SampaiTanggal = &t
	}

	res, err := h.svc.List(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, "list pembelian failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar pembelian")
	}

	gudangs, _ := h.gudangRepo.List(ctx, false)
	suppliers, _ := h.supplierSvc.List(ctx, repo.ListSupplierFilter{Page: 1, PerPage: 200})

	props := pembelianview.IndexProps{
		Nav:         layout.DefaultNav("/pembelian"),
		User:        userData(user),
		Items:       res.Items,
		Total:       res.Total,
		Page:        res.Page,
		PerPage:     res.PerPage,
		TotalPages:  res.TotalPages,
		SupplierID:  supplierID,
		GudangID:    gudangID,
		Status:      status,
		From:        from,
		To:          to,
		Gudangs:     gudangs,
		Suppliers:   suppliers.Items,
		FlashSuccess: c.QueryParam("flash"),
	}
	return RenderHTML(c, http.StatusOK, pembelianview.Index(props))
}

// New GET /pembelian/baru.
func (h *PembelianHandler) New(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()
	gudangs, _ := h.gudangRepo.List(ctx, false)
	suppliers, _ := h.supplierSvc.List(ctx, repo.ListSupplierFilter{Page: 1, PerPage: 200})
	satuans, _ := h.satuanRepo.List(ctx)

	props := pembelianview.FormProps{
		Nav:       layout.DefaultNav("/pembelian"),
		User:      userData(user),
		Gudangs:   gudangs,
		Suppliers: suppliers.Items,
		Satuans:   satuans,
		Input: dto.PembelianCreateInput{
			Tanggal:     time.Now().Format("2006-01-02"),
			StatusBayar: "lunas",
		},
	}
	return RenderHTML(c, http.StatusOK, pembelianview.Form(props))
}

// Create POST /pembelian.
func (h *PembelianHandler) Create(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	ctx := c.Request().Context()

	in := dto.PembelianCreateInput{}
	if err := c.Bind(&in); err != nil {
		return h.renderFormErr(c, user, in, nil, "Form tidak valid.")
	}
	// Bind items manual (multi-row form).
	in.Items = parseItemRows(c)

	if err := dto.Validate(&in); err != nil {
		fe, _ := dto.CollectFieldErrors(err)
		return h.renderFormErr(c, user, in, fe, "")
	}

	p, err := h.svc.Create(ctx, in, user.ID)
	if err != nil {
		slog.ErrorContext(ctx, "create pembelian failed", "error", err)
		return h.renderFormErr(c, user, in, nil, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/pembelian/"+strconv.FormatInt(p.ID, 10))
}

// Show GET /pembelian/:id.
func (h *PembelianHandler) Show(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID pembelian tidak valid")
	}
	ctx := c.Request().Context()

	p, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPembelianTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "Pembelian tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat pembelian")
	}

	hist, _ := h.svc.HistoryPembayaran(ctx, p.ID)
	sisa, _ := h.svc.SisaPembelian(ctx, p)

	props := pembelianview.ShowProps{
		Nav:         layout.DefaultNav("/pembelian"),
		User:        userData(user),
		Pembelian:   p,
		Pembayarans: hist,
		Sisa:        sisa,
	}
	return RenderHTML(c, http.StatusOK, pembelianview.Show(props))
}

// RecordPayment POST /pembelian/:id/bayar.
func (h *PembelianHandler) RecordPayment(c echo.Context) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "ID pembelian tidak valid")
	}
	ctx := c.Request().Context()

	p, err := h.svc.Get(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Pembelian tidak ditemukan")
	}

	in := dto.PembayaranSupplierInput{}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Form tidak valid")
	}
	idCopy := id
	in.PembelianID = &idCopy
	in.SupplierID = p.SupplierID

	if err := dto.Validate(&in); err != nil {
		// Render show with general error.
		hist, _ := h.svc.HistoryPembayaran(ctx, p.ID)
		sisa, _ := h.svc.SisaPembelian(ctx, p)
		return RenderHTML(c, http.StatusUnprocessableEntity, pembelianview.Show(pembelianview.ShowProps{
			Nav:         layout.DefaultNav("/pembelian"),
			User:        userData(user),
			Pembelian:   p,
			Pembayarans: hist,
			Sisa:        sisa,
			GeneralErr:  "Form pembayaran tidak valid: pastikan jumlah, tanggal, dan metode terisi.",
		}))
	}
	if _, err := h.svc.RecordPayment(ctx, in, user.ID); err != nil {
		slog.ErrorContext(ctx, "record payment failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mencatat pembayaran")
	}
	return c.Redirect(http.StatusSeeOther, "/pembelian/"+strconv.FormatInt(id, 10))
}

// --- helpers ---

func (h *PembelianHandler) renderFormErr(c echo.Context, user *auth.User, in dto.PembelianCreateInput, fe map[string]string, general string) error {
	ctx := c.Request().Context()
	gudangs, _ := h.gudangRepo.List(ctx, false)
	suppliers, _ := h.supplierSvc.List(ctx, repo.ListSupplierFilter{Page: 1, PerPage: 200})
	satuans, _ := h.satuanRepo.List(ctx)

	props := pembelianview.FormProps{
		Nav:       layout.DefaultNav("/pembelian"),
		User:      userData(user),
		Input:     in,
		Errors:    fe,
		General:   general,
		Gudangs:   gudangs,
		Suppliers: suppliers.Items,
		Satuans:   satuans,
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, pembelianview.Form(props))
}

// parseItemRows ambil items[] dari form (produk_id[], qty[], satuan_id[], harga_satuan[]).
func parseItemRows(c echo.Context) []dto.PembelianItemInput {
	form, _ := c.FormParams()
	produks := form["produk_id[]"]
	qtys := form["qty[]"]
	satuans := form["satuan_id[]"]
	hargas := form["harga_satuan[]"]
	n := len(produks)
	if n == 0 {
		return nil
	}
	out := make([]dto.PembelianItemInput, 0, n)
	for i := 0; i < n; i++ {
		pid, _ := strconv.ParseInt(safeIdx(produks, i), 10, 64)
		if pid <= 0 {
			continue
		}
		qty, _ := strconv.ParseFloat(safeIdx(qtys, i), 64)
		sid, _ := strconv.ParseInt(safeIdx(satuans, i), 10, 64)
		harga, _ := strconv.ParseInt(safeIdx(hargas, i), 10, 64)
		out = append(out, dto.PembelianItemInput{
			ProdukID:    pid,
			Qty:         qty,
			SatuanID:    sid,
			HargaSatuan: harga,
		})
	}
	return out
}

func safeIdx(arr []string, i int) string {
	if i < 0 || i >= len(arr) {
		return ""
	}
	return arr[i]
}

package handler

import (
	"context"
	"errors"
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
	produkview "github.com/omanjaya/tokobangunan/internal/view/produk"
)

// ProdukHandler - HTTP handler modul produk.
type ProdukHandler struct {
	produk *service.ProdukService
	satuan *service.SatuanService
	harga  *service.HargaService
}

func NewProdukHandler(p *service.ProdukService, s *service.SatuanService, h *service.HargaService) *ProdukHandler {
	return &ProdukHandler{produk: p, satuan: s, harga: h}
}

// Index GET /produk - list dengan filter & pagination.
func (h *ProdukHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()

	q := strings.TrimSpace(c.QueryParam("q"))
	kategori := strings.TrimSpace(c.QueryParam("kategori"))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	onlyActive := c.QueryParam("aktif") == "1"

	filter := repo.ListProdukFilter{
		Query:   q,
		Page:    page,
		PerPage: 25,
	}
	if kategori != "" {
		filter.Kategori = &kategori
	}
	if onlyActive {
		v := true
		filter.IsActive = &v
	}

	page1, err := h.produk.List(ctx, filter)
	if err != nil {
		return err
	}

	rows, err := h.buildRows(ctx, page1.Items)
	if err != nil {
		return err
	}

	kategoris, err := h.produk.ListKategori(ctx)
	if err != nil {
		return err
	}

	props := produkview.IndexProps{
		Nav:        layout.DefaultNav("/produk"),
		User:       userData(auth.CurrentUser(c)),
		Rows:       rows,
		Total:      page1.Total,
		Page:       page1.Page,
		PerPage:    page1.PerPage,
		TotalPages: page1.TotalPages,
		Query:      q,
		Kategori:   kategori,
		OnlyActive: onlyActive,
		Kategoris:  kategoris,
	}
	return RenderHTML(c, http.StatusOK, produkview.Index(props))
}

// New GET /produk/baru - form kosong.
func (h *ProdukHandler) New(c echo.Context) error {
	satuans, err := h.satuanLite(c.Request().Context())
	if err != nil {
		return err
	}
	props := produkview.FormProps{
		Nav:  layout.DefaultNav("/produk"),
		User: userData(auth.CurrentUser(c)),
		Input: dto.ProdukUpdateInput{
			IsActive:       true,
			FaktorKonversi: 1,
		},
		Satuans: satuans,
	}
	return RenderHTML(c, http.StatusOK, produkview.Form(props))
}

// Create POST /produk.
func (h *ProdukHandler) Create(c echo.Context) error {
	var in dto.ProdukCreateInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	in.IsActive = c.FormValue("is_active") == "true"

	ctx := c.Request().Context()
	created, err := h.produk.Create(ctx, in)
	if err != nil {
		return h.renderFormError(c, false, 0, toUpdateInput(in), err)
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/produk")
		return c.NoContent(http.StatusOK)
	}
	c.Response().Header().Set("Location", "/produk")
	_ = created
	return c.NoContent(http.StatusSeeOther)
}

// Edit GET /produk/:id/edit.
func (h *ProdukHandler) Edit(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.produk.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}
	satuans, err := h.satuanLite(ctx)
	if err != nil {
		return err
	}
	props := produkview.FormProps{
		Nav:     layout.DefaultNav("/produk"),
		User:    userData(auth.CurrentUser(c)),
		IsEdit:  true,
		ID:      p.ID,
		Input:   produkToInput(p),
		Satuans: satuans,
		FotoURL: p.FotoURL,
	}
	return RenderHTML(c, http.StatusOK, produkview.Form(props))
}

// Update POST /produk/:id.
func (h *ProdukHandler) Update(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var in dto.ProdukUpdateInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	in.IsActive = c.FormValue("is_active") == "true"

	if _, err := h.produk.Update(c.Request().Context(), id, in); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return h.renderConflict(c, true, id, in)
		}
		return h.renderFormError(c, true, id, in, err)
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/produk")
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, "/produk")
}

// Delete POST /produk/:id/delete.
func (h *ProdukHandler) Delete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.produk.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}
	// Untuk HTMX: balas empty string supaya row hilang.
	if c.Request().Header.Get("HX-Request") == "true" {
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, "/produk")
}

// GetBySKUJSON GET /produk/by-sku?sku=... - lookup by SKU untuk barcode scanner.
// Return JSON ringkas (id, sku, nama, harga eceran cents, stok info kosong).
func (h *ProdukHandler) GetBySKUJSON(c echo.Context) error {
	ctx := c.Request().Context()
	sku := strings.TrimSpace(c.QueryParam("sku"))
	if sku == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "sku wajib diisi")
	}
	// Cari via service Search (trigram + ILIKE), filter exact match SKU.
	items, err := h.produk.Search(ctx, sku, 5)
	if err != nil {
		return err
	}
	var match *domain.Produk
	for i := range items {
		if strings.EqualFold(items[i].SKU, sku) {
			x := items[i]
			match = &x
			break
		}
	}
	if match == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "produk tidak ditemukan", "sku": sku})
	}
	out := map[string]any{
		"id":              match.ID,
		"sku":             match.SKU,
		"nama":            match.Nama,
		"satuan_kecil_id": match.SatuanKecilID,
		"foto_url":        match.FotoURL,
	}
	if match.Kategori != nil {
		out["kategori"] = *match.Kategori
	}
	// Harga eceran (kalau ada).
	if hak, herr := h.harga.GetAktif(ctx, match.ID, nil, domain.TipeHargaEceran); herr == nil {
		out["harga_eceran"] = hak.HargaJual
	}
	return c.JSON(http.StatusOK, out)
}

// SearchAjax GET /produk/search?q=... - autocomplete partial.
func (h *ProdukHandler) SearchAjax(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	items, err := h.produk.Search(c.Request().Context(), q, limit)
	if err != nil {
		return err
	}
	return RenderHTML(c, http.StatusOK, produkview.SearchResults(produkview.SearchResultsProps{
		Items: items, Query: q,
	}))
}

// ----- helpers ---------------------------------------------------------------

func (h *ProdukHandler) buildRows(ctx context.Context, items []domain.Produk) ([]produkview.Row, error) {
	if len(items) == 0 {
		return nil, nil
	}
	satuanList, err := h.satuan.List(ctx)
	if err != nil {
		return nil, err
	}
	idx := map[int64]domain.Satuan{}
	for _, s := range satuanList {
		idx[s.ID] = s
	}

	out := make([]produkview.Row, 0, len(items))
	for i := range items {
		p := items[i]
		row := produkview.Row{Produk: p}
		if s, ok := idx[p.SatuanKecilID]; ok {
			row.SatuanKecil = s.Kode
		}
		if p.SatuanBesarID != nil {
			if s, ok := idx[*p.SatuanBesarID]; ok {
				row.SatuanBesar = s.Kode
			}
		}
		harga, err := h.harga.GetAktif(ctx, p.ID, nil, domain.TipeHargaEceran)
		if err == nil {
			row.HargaEceran = produkview.HargaSnapshot{Cents: harga.HargaJual, HasValue: true}
		} else if !errors.Is(err, domain.ErrHargaNotFound) {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (h *ProdukHandler) satuanLite(ctx context.Context) ([]produkview.SatuanLite, error) {
	list, err := h.satuan.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]produkview.SatuanLite, 0, len(list))
	for _, s := range list {
		out = append(out, produkview.SatuanLite{ID: s.ID, Kode: s.Kode, Nama: s.Nama})
	}
	return out, nil
}

// renderConflict - 409 dengan form rerender + alert refresh.
func (h *ProdukHandler) renderConflict(c echo.Context, isEdit bool, id int64, in dto.ProdukUpdateInput) error {
	satuans, lerr := h.satuanLite(c.Request().Context())
	if lerr != nil {
		return lerr
	}
	props := produkview.FormProps{
		Nav:     layout.DefaultNav("/produk"),
		User:    userData(auth.CurrentUser(c)),
		IsEdit:  isEdit,
		ID:      id,
		Input:   in,
		Satuans: satuans,
		General: "Data produk sudah diubah pengguna lain. Silakan refresh dan coba lagi.",
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		return RenderHTML(c, http.StatusConflict, produkview.FormCard(props))
	}
	return RenderHTML(c, http.StatusConflict, produkview.Form(props))
}

func (h *ProdukHandler) renderFormError(c echo.Context, isEdit bool, id int64, in dto.ProdukUpdateInput, err error) error {
	satuans, lerr := h.satuanLite(c.Request().Context())
	if lerr != nil {
		return lerr
	}
	props := produkview.FormProps{
		Nav:     layout.DefaultNav("/produk"),
		User:    userData(auth.CurrentUser(c)),
		IsEdit:  isEdit,
		ID:      id,
		Input:   in,
		Satuans: satuans,
	}
	if fes, ok := dto.CollectFieldErrors(err); ok {
		props.Errors = fes
	} else {
		props.General = humanizeProdukError(err)
	}
	status := http.StatusUnprocessableEntity
	// HTMX swap form saja.
	if c.Request().Header.Get("HX-Request") == "true" {
		return RenderHTML(c, status, produkview.FormCard(props))
	}
	return RenderHTML(c, status, produkview.Form(props))
}

func humanizeProdukError(err error) string {
	switch {
	case errors.Is(err, domain.ErrConflict):
		return "Data produk sudah diubah pengguna lain. Silakan refresh dan coba lagi."
	case errors.Is(err, domain.ErrSKUDuplikat):
		return "SKU sudah dipakai produk lain."
	case errors.Is(err, domain.ErrSKUWajib):
		return "SKU wajib diisi."
	case errors.Is(err, domain.ErrNamaWajib):
		return "Nama produk wajib diisi."
	case errors.Is(err, domain.ErrFaktorKonversiInvalid):
		return "Faktor konversi harus lebih besar dari 0."
	case errors.Is(err, domain.ErrStokMinimumInvalid):
		return "Stok minimum tidak boleh negatif."
	case errors.Is(err, domain.ErrSatuanNotFound):
		return "Satuan yang dipilih tidak ditemukan."
	case errors.Is(err, domain.ErrSatuanKecilWajib):
		return "Satuan kecil wajib dipilih."
	default:
		return "Gagal menyimpan produk: " + err.Error()
	}
}

func toUpdateInput(in dto.ProdukCreateInput) dto.ProdukUpdateInput {
	return dto.ProdukUpdateInput{
		SKU:            in.SKU,
		Nama:           in.Nama,
		Kategori:       in.Kategori,
		SatuanKecilID:  in.SatuanKecilID,
		SatuanBesarID:  in.SatuanBesarID,
		FaktorKonversi: in.FaktorKonversi,
		StokMinimum:    in.StokMinimum,
		IsActive:       in.IsActive,
	}
}

func produkToInput(p *domain.Produk) dto.ProdukUpdateInput {
	in := dto.ProdukUpdateInput{
		SKU:            p.SKU,
		Nama:           p.Nama,
		SatuanKecilID:  p.SatuanKecilID,
		FaktorKonversi: p.FaktorKonversi,
		StokMinimum:    p.StokMinimum,
		IsActive:       p.IsActive,
		Version:        p.Version,
	}
	if p.Kategori != nil {
		in.Kategori = *p.Kategori
	}
	if p.SatuanBesarID != nil {
		in.SatuanBesarID = *p.SatuanBesarID
	}
	return in
}

func pathID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	return id, nil
}


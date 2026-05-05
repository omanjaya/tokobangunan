package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/components"
)

// SearchHandler menyediakan global search topbar yang query 3 entitas:
// Produk (SKU/nama), Mitra (kode/nama), Penjualan (nomor kwitansi).
//
// Output utama adalah HTMX HTML partial (server-rendered dropdown). Tidak ada
// JSON karena consumer-nya hanya topbar dengan htmx swap ke #search-results.
type SearchHandler struct {
	produkSvc    *service.ProdukService
	mitraSvc     *service.MitraService
	penjualanSvc *service.PenjualanService
}

// NewSearchHandler constructor.
func NewSearchHandler(
	produkSvc *service.ProdukService,
	mitraSvc *service.MitraService,
	penjualanSvc *service.PenjualanService,
) *SearchHandler {
	return &SearchHandler{
		produkSvc:    produkSvc,
		mitraSvc:     mitraSvc,
		penjualanSvc: penjualanSvc,
	}
}

const searchPerCategory = 5

// Global GET /search?q=... — return Templ partial dropdown.
// Bila q < 2 chars, return dropdown kosong (HTMX akan swap kosong).
func (h *SearchHandler) Global(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	if len([]rune(q)) < 2 {
		// Kosongkan target.
		return c.HTML(http.StatusOK, "")
	}
	props := h.search(c.Request().Context(), q, searchPerCategory)
	return RenderHTML(c, http.StatusOK, components.SearchDropdown(props))
}

// FullPage GET /search/all?q=... — return halaman penuh (best-effort,
// fallback render same dropdown bila page belum tersedia).
func (h *SearchHandler) FullPage(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	props := h.search(c.Request().Context(), q, 50)

	// Karena kita tidak punya search.Results full page templ untuk saat ini
	// (avoid mengganggu agent paralel), reuse SearchDropdown dalam halaman
	// minimal — handler bisa di-upgrade nanti tanpa perubahan route.
	user := userData(nil)
	_ = user
	// Untuk full page kita pakai dropdown yang dibungkus div padded supaya
	// tetap bisa diakses langsung via URL. Ini bukan AppShell agar handler ini
	// tidak menyentuh wiring layout user/nav (ditangani route registrar yang
	// lebih kaya bila perlu).
	return RenderHTML(c, http.StatusOK, components.SearchDropdown(props))
}

// search core fan-out ke 3 service. Error per service di-isolasi: kalau salah
// satu gagal, list lain tetap dirender (degraded UX, bukan total fail).
func (h *SearchHandler) search(ctx context.Context, q string, limit int) components.SearchDropdownProps {
	props := components.SearchDropdownProps{Query: q}

	if h.produkSvc != nil {
		if list, err := h.produkSvc.Search(ctx, q, limit); err == nil {
			for _, p := range list {
				props.Produk = append(props.Produk, components.SearchHit{
					Type:      "produk",
					Href:      fmt.Sprintf("/produk/%d/edit", p.ID),
					Primary:   p.Nama,
					Secondary: p.SKU,
				})
			}
		}
	}

	if h.mitraSvc != nil {
		if list, err := h.mitraSvc.Search(ctx, q, limit); err == nil {
			for _, m := range list {
				props.Mitra = append(props.Mitra, components.SearchHit{
					Type:      "mitra",
					Href:      fmt.Sprintf("/mitra/%d", m.ID),
					Primary:   m.Nama,
					Secondary: m.Kode,
				})
			}
		}
	}

	if h.penjualanSvc != nil {
		if list, err := h.penjualanSvc.Search(ctx, q, limit); err == nil {
			for _, p := range list {
				props.Penjualan = append(props.Penjualan, components.SearchHit{
					Type:      "penjualan",
					Href:      fmt.Sprintf("/penjualan/%s", p.NomorKwitansi),
					Primary:   p.NomorKwitansi,
					Secondary: p.Tanggal.Format("2006-01-02"),
					Meta:      format.Rupiah(p.Total),
				})
			}
		}
	}

	return props
}

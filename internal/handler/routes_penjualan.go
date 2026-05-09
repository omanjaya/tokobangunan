package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// RegisterPenjualanRoutes mendaftarkan seluruh route modul penjualan.
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterPenjualanRoutes(g *echo.Group, ph *PenjualanHandler) {
	pj := g.Group("/penjualan")

	// Default /penjualan render POS (kasir cepat). Riwayat di /penjualan/list.
	pj.GET("", ph.New)
	pj.GET("/baru", ph.New)
	pj.GET("/list", ph.List)
	pj.POST("", ph.Create)

	// Autocomplete JSON (dipakai form Alpine.js).
	pj.GET("/search-mitra", ph.SearchMitraJSON)
	pj.GET("/search-produk", ph.SearchProdukJSON)
	pj.GET("/preview-nomor", ph.PreviewNomor)

	// Detail & cetak.
	pj.GET("/:id", ph.Show)
	pj.GET("/:id/print/pdf", ph.PrintPDF)
	pj.GET("/:id/print/faktur", ph.PrintFaktur)
	pj.GET("/:id/print/escp", ph.PrintDotMatrix)
	pj.GET("/:id/print/58mm", ph.PrintThermal58)
	pj.GET("/:id/print/80mm", ph.PrintThermal80)

	// Edit / Update / Cancel — owner & admin only.
	admin := pj.Group("", auth.RequireRole("owner", "admin"))
	admin.GET("/:id/edit", ph.Edit)
	admin.POST("/:id", ph.Update)
	admin.POST("/:id/cancel", ph.Cancel)
}

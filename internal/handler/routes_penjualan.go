package handler

import "github.com/labstack/echo/v4"

// RegisterPenjualanRoutes mendaftarkan seluruh route modul penjualan.
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterPenjualanRoutes(g *echo.Group, ph *PenjualanHandler) {
	pj := g.Group("/penjualan")

	pj.GET("", ph.Index)
	pj.GET("/baru", ph.New)
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
}

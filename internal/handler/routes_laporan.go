package handler

import "github.com/labstack/echo/v4"

// RegisterLaporanRoutes mendaftarkan seluruh route modul laporan.
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterLaporanRoutes(g *echo.Group, lh *LaporanHandler) {
	lap := g.Group("/laporan")

	lap.GET("", lh.Index)
	lap.GET("/lr", lh.LR)
	lap.GET("/penjualan", lh.Penjualan)
	lap.GET("/penjualan/export.csv", lh.ExportPenjualan)
	lap.GET("/penjualan/export.xlsx", lh.ExportPenjualanXLSX)
	lap.GET("/penjualan/export.pdf", lh.ExportPenjualanPDF)
	lap.GET("/mutasi", lh.Mutasi)
	lap.GET("/mutasi/export.pdf", lh.ExportMutasiPDF)
	lap.GET("/mutasi/export.xlsx", lh.ExportMutasiXLSX)
	lap.GET("/stok-kritis", lh.StokKritis)
	lap.GET("/stok-kritis/export.pdf", lh.ExportStokKritisPDF)
	lap.GET("/stok-kritis/export.xlsx", lh.ExportStokKritisXLSX)
	lap.GET("/top-produk", lh.TopProduk)
	lap.GET("/top-produk/export.pdf", lh.ExportTopProdukPDF)
	lap.GET("/top-produk/export.xlsx", lh.ExportTopProdukXLSX)
	lap.GET("/cashflow", lh.Cashflow)
	lap.GET("/cashflow/export.pdf", lh.ExportCashflowPDF)
	lap.GET("/cashflow/export.xlsx", lh.ExportCashflowXLSX)
	lap.GET("/lr/export.pdf", lh.ExportLRPDF)
	lap.GET("/lr/export.xlsx", lh.ExportLRXLSX)
}

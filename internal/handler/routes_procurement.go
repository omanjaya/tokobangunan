package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// RegisterProcurementRoutes mendaftarkan route modul pengadaan
// (pembelian + pembayaran supplier) dan stok opname.
//
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterProcurementRoutes(g *echo.Group, ph *PembelianHandler, oh *StokOpnameHandler) {
	pembelian := g.Group("/pembelian")
	pembelian.GET("", ph.Index)
	pembelian.GET("/baru", ph.New)
	pembelian.POST("", ph.Create)
	pembelian.GET("/:id", ph.Show)
	pembelian.POST("/:id/bayar", ph.RecordPayment)

	// Edit / Update / Cancel — owner & admin only.
	admin := pembelian.Group("", auth.RequireRole("owner", "admin"))
	admin.GET("/:id/edit", ph.Edit)
	admin.POST("/:id", ph.Update)
	admin.POST("/:id/cancel", ph.Cancel)

	opname := g.Group("/opname")
	opname.GET("", oh.Index)
	opname.GET("/baru", oh.New)
	opname.POST("", oh.Create)
	opname.GET("/:id", oh.Show)
	opname.POST("/:id/item/:produk_id", oh.UpdateItem)
	opname.POST("/:id/submit", oh.Submit)
	opname.POST("/:id/approve", oh.Approve)
}

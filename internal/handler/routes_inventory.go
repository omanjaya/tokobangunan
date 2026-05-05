package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// RegisterInventoryRoutes mendaftarkan route modul inventori (mutasi, stok,
// stok adjustment) ke group g (sudah memiliki middleware auth.RequireAuth).
func RegisterInventoryRoutes(g *echo.Group, mh *MutasiHandler, sh *StokHandler, ah *StokAdjustmentHandler) {
	mut := g.Group("/mutasi")
	mut.GET("", mh.Index)
	mut.GET("/baru", mh.New)
	mut.POST("", mh.Create)
	mut.GET("/:id", mh.Show)
	mut.POST("/:id/submit", mh.Submit)
	mut.POST("/:id/receive", mh.Receive)
	mut.POST("/:id/cancel", mh.Cancel)

	stok := g.Group("/stok")
	stok.GET("", sh.Index)
	stok.GET("/refresh", sh.RefreshAjax)
	stok.GET("/produk/:id", sh.Produk)

	// Stok adjustment — owner/admin only. DAFTAR SEBELUM /:gudang_id
	// supaya path "/adjust" tidak di-cocokkan ke param :gudang_id.
	if ah != nil {
		adj := stok.Group("/adjust", auth.RequireRole("owner", "admin"))
		adj.GET("", ah.New)
		adj.POST("", ah.Create)
		adj.GET("/history", ah.History)
	}

	stok.GET("/:gudang_id", sh.Detail)
}

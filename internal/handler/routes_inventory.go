package handler

import "github.com/labstack/echo/v4"

// RegisterInventoryRoutes mendaftarkan route modul inventori (mutasi & stok)
// ke group g (sudah memiliki middleware auth.RequireAuth dari main.go).
func RegisterInventoryRoutes(g *echo.Group, mh *MutasiHandler, sh *StokHandler) {
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
	stok.GET("/:gudang_id", sh.Detail)
}

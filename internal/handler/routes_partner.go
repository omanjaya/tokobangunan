package handler

import "github.com/labstack/echo/v4"

// RegisterPartnerRoutes mendaftarkan semua routes modul Mitra & Supplier ke
// group g (yang sudah memiliki middleware auth.RequireAuth dari main.go).
func RegisterPartnerRoutes(g *echo.Group, mh *MitraHandler, sh *SupplierHandler) {
	mitra := g.Group("/mitra")
	mitra.GET("", mh.Index)
	mitra.GET("/baru", mh.New)
	mitra.POST("", mh.Create)
	mitra.GET("/search", mh.SearchAjax)
	mitra.GET("/:id", mh.Show)
	mitra.GET("/:id/edit", mh.Edit)
	mitra.POST("/:id", mh.Update)
	mitra.POST("/:id/delete", mh.Delete)

	supplier := g.Group("/supplier")
	supplier.GET("", sh.Index)
	supplier.GET("/baru", sh.New)
	supplier.POST("", sh.Create)
	supplier.GET("/:id/edit", sh.Edit)
	supplier.POST("/:id", sh.Update)
	supplier.POST("/:id/delete", sh.Delete)
}

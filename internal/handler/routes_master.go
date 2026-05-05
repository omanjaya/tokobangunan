package handler

import (
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/labstack/echo/v4"
)

// RegisterMasterRoutes mendaftarkan seluruh route modul master data
// (produk, satuan, harga produk) di group yang sudah dilindungi auth.
//
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterMasterRoutes(g *echo.Group, p *ProdukHandler, s *SatuanHandler, h *HargaHandler, pf *ProdukFotoHandler, pl *ProdukLabelHandler) {
	// Produk -------------------------------------------------------------
	g.GET("/produk", p.Index)
	g.GET("/produk/baru", p.New)
	g.POST("/produk", p.Create)
	g.GET("/produk/search", p.SearchAjax)
	g.GET("/produk/by-sku", p.GetBySKUJSON)
	g.GET("/produk/:id/edit", p.Edit)
	g.POST("/produk/:id", p.Update)
	g.POST("/produk/:id/delete", p.Delete)

	// Foto produk (upload terbatas 3 MB body).
	uploadLimit := echomw.BodyLimit("3M")
	g.POST("/produk/:id/foto", pf.Upload, uploadLimit)
	g.POST("/produk/:id/foto/delete", pf.Delete)

	// Label / barcode produk.
	g.GET("/produk/:id/label", pl.LabelPDF)

	// Harga per produk ---------------------------------------------------
	g.GET("/produk/:id/harga", h.Index)
	g.POST("/produk/:id/harga", h.Set)

	// Satuan -------------------------------------------------------------
	g.GET("/satuan", s.Index)
	g.POST("/satuan", s.Create)
}

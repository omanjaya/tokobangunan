package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RegisterCollectionRoutes mendaftarkan routes pembayaran customer (mitra),
// piutang aging, dan tabungan mitra.
//
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
//
// Catatan: route /mitra/:id/* ditambahkan ke group "/mitra" agar tidak konflik
// dengan registration di routes_partner.go (Echo route tree menerima sub-path
// terpisah selama param name konsisten).
func RegisterCollectionRoutes(g *echo.Group, ph *PembayaranHandler, ph2 *PiutangHandler, th *TabunganHandler) {
	pembayaran := g.Group("/pembayaran")
	pembayaran.GET("", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/piutang")
	})
	pembayaran.POST("", ph.Record)
	pembayaran.POST("/batch", ph.RecordBatch)

	piutang := g.Group("/piutang")
	piutang.GET("", ph2.Index)
	piutang.GET("/:mitra_id", ph2.MitraDetail)

	mitra := g.Group("/mitra")
	mitra.GET("/:id/pembayaran", ph.MitraHistory)
	mitra.GET("/:id/tabungan", th.Show)
	mitra.POST("/:id/tabungan/setor", th.Setor)
	mitra.POST("/:id/tabungan/tarik", th.Tarik)
}

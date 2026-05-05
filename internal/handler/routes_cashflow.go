package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// RegisterCashflowRoutes mendaftarkan route modul kas.
// Group g sebaiknya sudah memiliki middleware auth.RequireAuth.
func RegisterCashflowRoutes(g *echo.Group, ch *CashflowHandler) {
	kg := g.Group("/kas")
	kg.GET("", ch.Index)
	kg.GET("/baru", ch.New)
	kg.POST("", ch.Create)
	kg.GET("/:id", ch.Show)
	kg.POST("/:id/delete", ch.Delete, auth.RequireRole("owner"))
}

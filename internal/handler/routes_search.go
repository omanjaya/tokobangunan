package handler

import "github.com/labstack/echo/v4"

// RegisterSearchRoutes mendaftarkan endpoint global search ke group g
// (yang sudah memiliki middleware auth.RequireAuth dari main.go).
//
//	GET /search       — HTMX partial dropdown
//	GET /search/all   — halaman penuh hasil search (fallback)
func RegisterSearchRoutes(g *echo.Group, sh *SearchHandler) {
	g.GET("/search", sh.Global)
	g.GET("/search/all", sh.FullPage)
}

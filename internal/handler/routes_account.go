package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// RegisterAccountRoutes mendaftarkan route self-service akun + audit log.
// `g` HARUS sudah dipasang RequireAuth.
//
// Routes:
//   - GET  /profil
//   - POST /profil
//   - GET  /profil/password
//   - POST /profil/password
//   - GET  /help/shortcuts
//   - GET  /setting/audit-log         (owner/admin)
//   - GET  /setting/audit-log/:id     (owner/admin)
func RegisterAccountRoutes(g *echo.Group, ph *ProfileHandler, ah *AuditLogHandler, hh *HelpHandler) {
	g.GET("/profil", ph.Show)
	g.POST("/profil", ph.Update)
	g.GET("/profil/password", ph.ShowChangePassword)
	g.POST("/profil/password", ph.ChangePassword)

	g.GET("/help/shortcuts", hh.Shortcuts)

	audit := g.Group("/setting/audit-log", auth.RequireRole("owner", "admin"))
	audit.GET("", ah.Index)
	audit.GET("/:id", ah.Show)
}

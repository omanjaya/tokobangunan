package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/view/help"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// HelpHandler - dokumentasi keyboard shortcut & info help.
type HelpHandler struct{}

func NewHelpHandler() *HelpHandler {
	return &HelpHandler{}
}

// Shortcuts GET /help/shortcuts
func (h *HelpHandler) Shortcuts(c echo.Context) error {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav("/help/shortcuts")
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	return RenderHTML(c, http.StatusOK, help.Shortcuts(help.ShortcutsProps{
		Nav: nav, User: ud,
	}))
}

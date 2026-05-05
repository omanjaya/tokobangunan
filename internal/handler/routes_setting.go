package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	"github.com/omanjaya/tokobangunan/internal/view/setting"
)

// RegisterSettingRoutes mendaftarkan route /setting di bawah grup `g` yang
// HARUS sudah dipasang RequireAuth. Sub-route /setting dibatasi role owner+admin.
func RegisterSettingRoutes(g *echo.Group, gh *GudangHandler, uh *UserAccountHandler, gudangRepo *repo.GudangRepo) {
	s := g.Group("/setting", auth.RequireRole("owner", "admin"))
	s.GET("", settingIndex(gudangRepo))

	gud := s.Group("/gudang")
	gud.GET("", gh.Index)
	gud.GET("/baru", gh.New)
	gud.POST("", gh.Create)
	gud.GET("/:id/edit", gh.Edit)
	gud.POST("/:id", gh.Update)
	gud.POST("/:id/toggle-active", gh.ToggleActive)

	usr := s.Group("/user")
	usr.GET("", uh.Index)
	usr.GET("/baru", uh.New)
	usr.POST("", uh.Create)
	usr.GET("/:id/edit", uh.Edit)
	usr.POST("/:id", uh.Update)
	usr.POST("/:id/reset-password", uh.ResetPassword)
	usr.POST("/:id/toggle-active", uh.ToggleActive)
}

// settingIndex render halaman landing /setting (grid sub-menu).
func settingIndex(gudangRepo *repo.GudangRepo) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		gudangCount := safeGudangCount(ctx, gudangRepo)

		user := auth.CurrentUser(c)
		nav := layout.DefaultNav("/setting")
		ud := layout.UserData{}
		if user != nil {
			ud.Name = user.NamaLengkap
			ud.Role = user.Role
		}

		return RenderHTML(c, http.StatusOK, setting.Index(setting.IndexProps{
			Nav:         nav,
			User:        ud,
			GudangCount: gudangCount,
		}))
	}
}

func safeGudangCount(ctx context.Context, r *repo.GudangRepo) int {
	items, err := r.List(ctx, true)
	if err != nil {
		slog.WarnContext(ctx, "count gudang", "error", err)
		return 0
	}
	return len(items)
}

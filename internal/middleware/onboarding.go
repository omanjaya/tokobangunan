package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// RequireOnboarding redirect owner ke /onboarding kalau belum selesai setup.
// Skip untuk path /onboarding/*, /logout, /static/*, /healthz.
func RequireOnboarding(svc *service.AppSettingService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if isOnboardingExempt(path) {
				return next(c)
			}
			user := auth.CurrentUser(c)
			if user == nil || user.Role != "owner" {
				return next(c)
			}
			done, err := svc.IsOnboardingDone(c.Request().Context())
			if err != nil {
				// Jangan blok kalau check error.
				return next(c)
			}
			if done {
				return next(c)
			}
			if c.Request().Header.Get("HX-Request") == "true" {
				c.Response().Header().Set("HX-Redirect", "/onboarding")
				return c.NoContent(http.StatusOK)
			}
			return c.Redirect(http.StatusSeeOther, "/onboarding")
		}
	}
}

func isOnboardingExempt(path string) bool {
	switch {
	case strings.HasPrefix(path, "/onboarding"):
		return true
	case path == "/logout":
		return true
	case strings.HasPrefix(path, "/static/"):
		return true
	case path == "/healthz":
		return true
	case path == "/sw.js" || path == "/manifest.webmanifest":
		return true
	}
	return false
}

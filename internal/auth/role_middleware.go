package auth

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"
)

// RequireRole membatasi akses route hanya untuk role tertentu. Harus dipasang
// SETELAH RequireAuth supaya CurrentUser tersedia.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := CurrentUser(c)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			if !slices.Contains(roles, user.Role) {
				return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
			}
			return next(c)
		}
	}
}

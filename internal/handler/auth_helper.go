package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
)

// isPrivilegedRole - true kalau role boleh akses lintas gudang.
func isPrivilegedRole(role string) bool {
	return role == "owner" || role == "admin"
}

// enforceGudangScope memastikan user yang sedang login berhak mengakses
// resource di gudang tertentu. Owner/admin boleh akses semua. User lain
// hanya boleh akses gudang yang sama dengan user.GudangID.
//
// Mengembalikan 401 kalau belum login, 403 kalau gudang berbeda.
func enforceGudangScope(c echo.Context, resourceGudangID int64) error {
	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
	}
	if isPrivilegedRole(u.Role) {
		return nil
	}
	if u.GudangID != nil && *u.GudangID == resourceGudangID {
		return nil
	}
	return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
}

// enforceGudangScopeAny memastikan user berhak akses kalau resource berkaitan
// dengan SALAH SATU dari beberapa gudang (mis. mutasi: asal ATAU tujuan).
func enforceGudangScopeAny(c echo.Context, resourceGudangIDs ...int64) error {
	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
	}
	if isPrivilegedRole(u.Role) {
		return nil
	}
	if u.GudangID == nil {
		return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
	}
	for _, g := range resourceGudangIDs {
		if *u.GudangID == g {
			return nil
		}
	}
	return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
}

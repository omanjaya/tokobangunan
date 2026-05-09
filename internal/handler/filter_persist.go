package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// filterPersistMaxAge umur cookie filter (1 minggu).
const filterPersistMaxAge = 7 * 24 * 3600

// filterPersist memberi efek "remember last filter" pada list page.
//
// Cara pakai (di awal handler list):
//
//	if redir, t := filterPersist(c, "tb_filter_penjualan"); redir {
//	    return c.Redirect(http.StatusSeeOther, t)
//	}
//
// Behavior:
//   - Query ?reset=1   → hapus cookie, redirect ke path tanpa query.
//   - Query kosong + cookie ada → redirect ke path?<saved-query>.
//   - Query ada        → simpan ke cookie (path-scoped, HttpOnly, SameSite=Lax).
//
// Cookie value berupa raw query string, mis: "from=2025-11-01&to=2025-11-30&gudang_id=2".
func filterPersist(c echo.Context, cookieName string) (redirect bool, target string) {
	path := c.Request().URL.Path

	// Reset: clear cookie + bersih ke path tanpa query.
	if c.QueryParam("reset") == "1" {
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     path,
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		return true, path
	}

	q := c.Request().URL.RawQuery
	if q == "" {
		ck, err := c.Cookie(cookieName)
		if err == nil && ck != nil && ck.Value != "" {
			return true, path + "?" + ck.Value
		}
		return false, ""
	}

	c.SetCookie(&http.Cookie{
		Name:     cookieName,
		Value:    q,
		Path:     path,
		MaxAge:   filterPersistMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return false, ""
}

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// MitraAccessHandler - owner-side handler untuk magic link mitra portal.
type MitraAccessHandler struct {
	access *service.MitraAccessService
}

// NewMitraAccessHandler konstruktor.
func NewMitraAccessHandler(s *service.MitraAccessService) *MitraAccessHandler {
	return &MitraAccessHandler{access: s}
}

// CreateLink POST /mitra/:id/access-link?expires_days=30
// Return JSON {token, url, expires_at}.
func (h *MitraAccessHandler) CreateLink(c echo.Context) error {
	mitraID, err := pathID(c)
	if err != nil {
		return err
	}
	days, _ := strconv.Atoi(c.QueryParam("expires_days"))
	if days <= 0 {
		days = 30
	}
	t, err := h.access.Create(c.Request().Context(), mitraID, days)
	if err != nil {
		if errors.Is(err, domain.ErrMitraTidakDitemukan) {
			return echo.NewHTTPError(http.StatusNotFound, "mitra tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rel := "/portal/" + t.Token
	return c.JSON(http.StatusOK, echo.Map{
		"token":      t.Token,
		"url":        buildAbsoluteURL(c, rel),
		"path":       rel,
		"expires_at": t.ExpiresAt.Format("2006-01-02 15:04:05"),
	})
}

// RevokeLink POST /mitra/:id/access-link/:tokenID/revoke.
func (h *MitraAccessHandler) RevokeLink(c echo.Context) error {
	mitraID, err := pathID(c)
	if err != nil {
		return err
	}
	tokenID, err := strconv.ParseInt(c.Param("tokenID"), 10, 64)
	if err != nil || tokenID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "token id tidak valid")
	}
	if err := h.access.Revoke(c.Request().Context(), tokenID, mitraID); err != nil {
		if errors.Is(err, domain.ErrAccessTokenNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "token tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, echo.Map{"success": true})
}

// ListLinks GET /mitra/:id/access-tokens — JSON list token milik mitra.
func (h *MitraAccessHandler) ListLinks(c echo.Context) error {
	mitraID, err := pathID(c)
	if err != nil {
		return err
	}
	items, err := h.access.ListByMitra(c.Request().Context(), mitraID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		rel := "/portal/" + t.Token
		out = append(out, map[string]any{
			"id":         t.ID,
			"url":        buildAbsoluteURL(c, rel),
			"created_at": t.CreatedAt.Format("2006-01-02 15:04"),
			"expires_at": t.ExpiresAt.Format("2006-01-02 15:04"),
			"revoked":    t.Revoked,
			"valid":      t.IsValid() == nil,
		})
	}
	return c.JSON(http.StatusOK, out)
}

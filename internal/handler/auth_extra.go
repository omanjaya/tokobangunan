package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	authview "github.com/omanjaya/tokobangunan/internal/view/auth"
)

// AuthExtraHandler menangani route auth tambahan yang tidak dimiliki AuthHandler
// (mis. lupa password). Dipisah supaya tidak menyentuh AuthHandler existing.
type AuthExtraHandler struct{}

func NewAuthExtraHandler() *AuthExtraHandler {
	return &AuthExtraHandler{}
}

// ShowForgotPassword GET /lupa-password
// Fase 1: tampilkan instruksi statis (hubungi admin/owner). Belum ada flow
// reset via email.
func (h *AuthExtraHandler) ShowForgotPassword(c echo.Context) error {
	return RenderHTML(c, http.StatusOK, authview.ForgotPassword())
}

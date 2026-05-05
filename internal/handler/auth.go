package handler

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	authview "github.com/omanjaya/tokobangunan/internal/view/auth"
)

type AuthHandler struct {
	store  *auth.Store
	secure bool
}

func NewAuthHandler(store *auth.Store, secure bool) *AuthHandler {
	return &AuthHandler{store: store, secure: secure}
}

// ShowLogin GET /login - render halaman login.
func (h *AuthHandler) ShowLogin(c echo.Context) error {
	csrfToken, _ := c.Get("csrf").(string)
	return render(c, http.StatusOK, authview.Login(authview.LoginProps{
		CSRFToken: csrfToken,
	}))
}

// Login POST /login - validate credential, create session.
func (h *AuthHandler) Login(c echo.Context) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	csrfToken, _ := c.Get("csrf").(string)

	if username == "" || password == "" {
		return render(c, http.StatusUnprocessableEntity, authview.Login(authview.LoginProps{
			Username:  username,
			CSRFToken: csrfToken,
			Error:     "Username dan password wajib diisi.",
		}))
	}

	ctx := c.Request().Context()
	user, err := h.store.Authenticate(ctx, username, password)
	if err != nil {
		var msg string
		switch {
		case errors.Is(err, auth.ErrInvalidCredential):
			msg = "Username atau password salah."
		case errors.Is(err, auth.ErrUserLocked):
			msg = "Akun dikunci sementara karena terlalu banyak percobaan gagal. Coba lagi nanti."
		default:
			slog.ErrorContext(ctx, "authenticate failed", "error", err)
			msg = "Terjadi kesalahan, silakan coba lagi."
		}
		return render(c, http.StatusUnauthorized, authview.Login(authview.LoginProps{
			Username:  username,
			CSRFToken: csrfToken,
			Error:     msg,
		}))
	}

	ip := remoteIP(c.Request().RemoteAddr)
	userAgent := c.Request().UserAgent()
	sess, err := h.store.CreateSession(ctx, user.ID, ip, userAgent)
	if err != nil {
		slog.ErrorContext(ctx, "create session failed", "error", err)
		return render(c, http.StatusInternalServerError, authview.Login(authview.LoginProps{
			Username:  username,
			CSRFToken: csrfToken,
			Error:     "Gagal membuat sesi. Silakan coba lagi.",
		}))
	}

	auth.SetSessionCookie(c, sess.ID, h.secure)

	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/dashboard")
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

// Logout POST /logout - hapus session, clear cookie.
func (h *AuthHandler) Logout(c echo.Context) error {
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		if id, err := uuid.Parse(cookie.Value); err == nil {
			_ = h.store.DeleteSession(c.Request().Context(), id)
		}
	}
	auth.ClearSessionCookie(c, h.secure)
	return c.Redirect(http.StatusSeeOther, "/login")
}

func remoteIP(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(host)
}

// render menulis Templ component sebagai HTML response.
func render(c echo.Context, status int, t templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return t.Render(c.Request().Context(), c.Response().Writer)
}

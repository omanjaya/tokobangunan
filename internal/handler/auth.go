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
	"github.com/omanjaya/tokobangunan/internal/service"
	authview "github.com/omanjaya/tokobangunan/internal/view/auth"
)

type AuthHandler struct {
	store  *auth.Store
	secure bool
	audit  *service.AuditLogService // optional; nil-safe
}

func NewAuthHandler(store *auth.Store, secure bool, audit *service.AuditLogService) *AuthHandler {
	return &AuthHandler{store: store, secure: secure, audit: audit}
}

// recordAuditAuth - best-effort audit untuk event auth. userID 0 -> nil (unknown).
func (h *AuthHandler) recordAuditAuth(c echo.Context, userID int64, aksi string, payload map[string]any) {
	if h.audit == nil {
		return
	}
	var uid *int64
	if userID > 0 {
		id := userID
		uid = &id
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["ip"] = c.RealIP()
	payload["user_agent"] = c.Request().UserAgent()
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	if reqID == "" {
		reqID = c.Request().Header.Get(echo.HeaderXRequestID)
	}
	_ = h.audit.Record(c.Request().Context(), service.RecordEntry{
		UserID:    uid,
		Aksi:      aksi,
		Tabel:     "auth",
		RecordID:  userID,
		After:     payload,
		IP:        c.RealIP(),
		UserAgent: c.Request().UserAgent(),
		RequestID: reqID,
	})
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
		// Generic message untuk semua kasus credential (mitigasi user enumeration
		// & info leak akun-terkunci). Detail granular hanya di log internal.
		const genericMsg = "Username atau password salah."
		msg := genericMsg
		status := http.StatusUnauthorized
		reason := "invalid_credential"
		switch {
		case errors.Is(err, auth.ErrInvalidCredential):
			slog.WarnContext(ctx, "login failed: invalid credential",
				"username", username, "remote", c.Request().RemoteAddr)
		case errors.Is(err, auth.ErrUserLocked):
			slog.WarnContext(ctx, "login failed: account locked",
				"username", username, "remote", c.Request().RemoteAddr)
			reason = "user_locked"
		default:
			slog.ErrorContext(ctx, "authenticate failed",
				"error", err, "username", username)
			msg = "Terjadi kesalahan, silakan coba lagi."
			status = http.StatusInternalServerError
			reason = "error"
		}
		h.recordAuditAuth(c, 0, "login_failed", map[string]any{
			"username": username,
			"reason":   reason,
		})
		return render(c, status, authview.Login(authview.LoginProps{
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
	h.recordAuditAuth(c, user.ID, "login", map[string]any{
		"username": username,
	})

	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/dashboard")
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

// Logout POST /logout - hapus session, clear cookie.
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()
	var auditUserID int64
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		if id, err := uuid.Parse(cookie.Value); err == nil {
			if sess, lookupErr := h.store.GetSession(ctx, id); lookupErr == nil && sess != nil {
				auditUserID = sess.UserID
			}
			_ = h.store.DeleteSession(ctx, id)
		}
	}
	if auditUserID > 0 {
		h.recordAuditAuth(c, auditUserID, "logout", nil)
	}
	auth.ClearSessionCookie(c, h.secure)
	// Clear-Site-Data: instruksi browser untuk hapus storage (localStorage,
	// sessionStorage, IndexedDB), cache, dan cookie. Hanya berlaku di HTTPS
	// untuk sebagian besar browser; di HTTP modern Chrome/Firefox akan
	// mengabaikan tanpa error. Sebagai fallback, query ?logout=1 dipakai
	// JS untuk cleanup di sisi klien (lihat web/static/js/app.js).
	c.Response().Header().Set("Clear-Site-Data", `"storage", "cache"`)
	return c.Redirect(http.StatusSeeOther, "/login?logout=1")
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

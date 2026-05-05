package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	SessionCookieName = "session_id"
	contextUserKey    = "auth.user"
)

// SetSessionCookie tulis cookie session ke response.
func SetSessionCookie(c echo.Context, sessionID uuid.UUID, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionDuration.Seconds()),
	})
}

// ClearSessionCookie menghapus cookie session.
func ClearSessionCookie(c echo.Context, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// ContextWithUser menyimpan user di context request.
func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

type contextKey struct{}

// UserFromContext mengambil user yang sudah login dari context. Nil jika tidak ada.
func UserFromContext(ctx context.Context) *User {
	v, _ := ctx.Value(contextKey{}).(*User)
	return v
}

// CurrentUser ambil user dari Echo context.
func CurrentUser(c echo.Context) *User {
	v, ok := c.Get(contextUserKey).(*User)
	if !ok {
		return nil
	}
	return v
}

// RequireAuth middleware yang validasi cookie session, load user, simpan ke context.
// Kalau tidak ada session valid, redirect ke /login (untuk GET) atau 401 (lainnya).
func RequireAuth(store *Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				return rejectUnauthenticated(c)
			}

			id, err := uuid.Parse(cookie.Value)
			if err != nil {
				return rejectUnauthenticated(c)
			}

			ctx := c.Request().Context()
			sess, err := store.GetSession(ctx, id)
			if err != nil {
				if errors.Is(err, ErrSessionExpired) {
					return rejectUnauthenticated(c)
				}
				return err
			}

			user, err := store.GetUserByID(ctx, sess.UserID)
			if err != nil {
				return rejectUnauthenticated(c)
			}
			if !user.IsActive {
				_ = store.DeleteSession(ctx, sess.ID)
				return rejectUnauthenticated(c)
			}

			// Sliding window: refresh kalau sisa < 50%.
			if time.Until(sess.ExpiresAt) < SessionDuration/2 {
				_ = store.RefreshSession(ctx, sess.ID)
			}

			c.Set(contextUserKey, user)
			c.SetRequest(c.Request().WithContext(ContextWithUser(ctx, user)))
			return next(c)
		}
	}
}

// RedirectIfAuth middleware buat halaman publik (login). Kalau user sudah login,
// redirect ke dashboard.
func RedirectIfAuth(store *Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				return next(c)
			}
			id, err := uuid.Parse(cookie.Value)
			if err != nil {
				return next(c)
			}
			if _, err := store.GetSession(c.Request().Context(), id); err == nil {
				return c.Redirect(http.StatusSeeOther, "/dashboard")
			}
			return next(c)
		}
	}
}

func rejectUnauthenticated(c echo.Context) error {
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/login")
		return c.NoContent(http.StatusOK)
	}
	if c.Request().Method == http.MethodGet {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	return echo.NewHTTPError(http.StatusUnauthorized, "tidak terautentikasi")
}

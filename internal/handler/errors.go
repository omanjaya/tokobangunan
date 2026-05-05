package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	errorsview "github.com/omanjaya/tokobangunan/internal/view/errors"
)

// NewErrorHandler mengembalikan custom HTTPErrorHandler Echo yang merender
// halaman 404/403/500 dengan AuthLayout. Wiring di main.go:
//
//	e.HTTPErrorHandler = handler.NewErrorHandler()
//
// Untuk request HTMX (header HX-Request) tetap kembalikan plain text supaya
// fragment swap tidak rusak; untuk request JSON (Accept: application/json)
// kembalikan JSON sederhana.
func NewErrorHandler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status := http.StatusInternalServerError
		msg := "Terjadi kesalahan internal."
		var he *echo.HTTPError
		if errors.As(err, &he) {
			status = he.Code
			if m, ok := he.Message.(string); ok && m != "" {
				msg = m
			}
		}

		// Log non-404 saja agar log tidak banjir.
		if status >= 500 {
			slog.ErrorContext(c.Request().Context(), "http error",
				"status", status, "path", c.Path(), "error", err)
		}

		req := c.Request()
		if req.Header.Get("HX-Request") == "true" {
			c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
			c.Response().WriteHeader(status)
			_, _ = c.Response().Write([]byte(msg))
			return
		}
		if accept := req.Header.Get("Accept"); accept == echo.MIMEApplicationJSON ||
			accept == "application/json" {
			_ = c.JSON(status, echo.Map{"status": status, "error": msg})
			return
		}

		var page = errorsview.ServerError(msg)
		switch status {
		case http.StatusNotFound:
			page = errorsview.NotFound(msg)
		case http.StatusForbidden:
			page = errorsview.Forbidden(msg)
		case http.StatusUnauthorized:
			// Tidak override behavior unauth (auth middleware sudah redirect),
			// tapi kalau sampai sini render forbidden-style.
			page = errorsview.Forbidden(msg)
		}
		_ = RenderHTML(c, status, page)
	}
}

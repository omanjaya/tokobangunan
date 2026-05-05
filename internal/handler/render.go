package handler

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// RenderHTML menulis Templ component sebagai HTML response.
// Exported supaya handler di package lain (atau test) bisa pakai.
func RenderHTML(c echo.Context, status int, t templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return t.Render(c.Request().Context(), c.Response().Writer)
}

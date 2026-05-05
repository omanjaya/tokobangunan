package handler

import (
	"encoding/json"

	"github.com/labstack/echo/v4"
)

// ToastVariant adalah whitelist tipe toast yang konsisten dengan styling
// Alpine ToastContainer di layout.templ.
type ToastVariant string

const (
	ToastSuccess ToastVariant = "success"
	ToastError   ToastVariant = "error"
	ToastInfo    ToastVariant = "info"
	ToastWarning ToastVariant = "warning"
)

// TriggerToast set HX-Trigger header agar HTMX dispatch event "showToast"
// di sisi klien. Format payload mengikuti spec HTMX:
//
//	HX-Trigger: {"showToast": {"variant":"success","title":"...","message":"..."}}
//
// Listener di web/static/js/app.js akan menerjemahkan ke window event
// "toast:add" yang dirender oleh ToastContainer Alpine.
func TriggerToast(c echo.Context, variant ToastVariant, title, message string) {
	payload := map[string]any{
		"showToast": map[string]string{
			"variant": string(variant),
			"title":   title,
			"message": message,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// fallback string sederhana — sangat tidak mungkin karena map literal.
		c.Response().Header().Set("HX-Trigger", "showToast")
		return
	}
	c.Response().Header().Set("HX-Trigger", string(b))
}

// TriggerToastSuccess shortcut.
func TriggerToastSuccess(c echo.Context, title, message string) {
	TriggerToast(c, ToastSuccess, title, message)
}

// TriggerToastError shortcut.
func TriggerToastError(c echo.Context, title, message string) {
	TriggerToast(c, ToastError, title, message)
}

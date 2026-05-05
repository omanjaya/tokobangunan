package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/email"
	"github.com/omanjaya/tokobangunan/internal/service"
	settingview "github.com/omanjaya/tokobangunan/internal/view/setting"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// SettingSMTPHandler - owner-only edit SMTP config.
type SettingSMTPHandler struct {
	appSetting *service.AppSettingService
}

// NewSettingSMTPHandler konstruktor.
func NewSettingSMTPHandler(as *service.AppSettingService) *SettingSMTPHandler {
	return &SettingSMTPHandler{appSetting: as}
}

// Show GET /setting/smtp.
func (h *SettingSMTPHandler) Show(c echo.Context) error {
	cfg, _ := h.appSetting.SMTPConfig(c.Request().Context())
	flash := c.QueryParam("flash")
	errMsg := c.QueryParam("error")

	props := settingview.SMTPProps{
		Nav:   layout.DefaultNav("/setting"),
		User:  smtpUserData(c),
		Cfg:   cfg,
		Flash: flash,
		Error: errMsg,
	}
	if csrf, ok := c.Get("csrf").(string); ok {
		props.CSRFToken = csrf
	}
	return RenderHTML(c, http.StatusOK, settingview.SMTP(props))
}

// Update POST /setting/smtp.
func (h *SettingSMTPHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	user := auth.CurrentUser(c)
	var userID *int64
	if user != nil {
		uid := user.ID
		userID = &uid
	}

	// Load existing for password preserve.
	existing, _ := h.appSetting.SMTPConfig(ctx)

	cfg := &domain.SMTPConfig{
		Host:     strings.TrimSpace(c.FormValue("host")),
		Port:     strings.TrimSpace(c.FormValue("port")),
		Username: strings.TrimSpace(c.FormValue("username")),
		Password: c.FormValue("password"),
		From:     strings.TrimSpace(c.FormValue("from")),
		Enabled:  c.FormValue("enabled") == "true",
	}
	if cfg.Password == "" && existing != nil {
		cfg.Password = existing.Password
	}

	if err := h.appSetting.UpdateSMTPConfig(ctx, cfg, userID); err != nil {
		return c.Redirect(http.StatusSeeOther, "/setting/smtp?error="+err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/setting/smtp?flash=Konfigurasi+SMTP+disimpan")
}

// Test POST /setting/smtp/test - kirim email test pakai konfigurasi tersimpan.
func (h *SettingSMTPHandler) Test(c echo.Context) error {
	to := strings.TrimSpace(c.FormValue("test_email"))
	if to == "" {
		return c.Redirect(http.StatusSeeOther, "/setting/smtp?error=Email+test+wajib+diisi")
	}
	cfg, err := h.appSetting.SMTPConfig(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/setting/smtp?error="+err.Error())
	}
	if cfg == nil || !cfg.Enabled {
		return c.Redirect(http.StatusSeeOther, "/setting/smtp?error=SMTP+belum+diaktifkan")
	}
	sender := email.NewSender(email.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
	})
	err = sender.Send(email.Message{
		To:       to,
		Subject:  "Test Email - Tokobangunan",
		BodyText: "Ini email test dari aplikasi Tokobangunan. Jika Anda menerima email ini, konfigurasi SMTP sudah berfungsi.",
		BodyHTML: "<p>Ini email <strong>test</strong> dari aplikasi Tokobangunan.</p><p>Jika Anda menerima email ini, konfigurasi SMTP sudah berfungsi.</p>",
	})
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/setting/smtp?error=Gagal+kirim:+"+err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/setting/smtp?flash=Email+test+terkirim+ke+"+to)
}

func smtpUserData(c echo.Context) layout.UserData {
	u := auth.CurrentUser(c)
	if u == nil {
		return layout.UserData{}
	}
	return layout.UserData{Name: u.NamaLengkap, Role: u.Role}
}

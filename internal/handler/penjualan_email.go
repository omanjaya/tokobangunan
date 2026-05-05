package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/email"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/print/pdf"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// PenjualanEmailHandler kirim PDF kwitansi via email.
type PenjualanEmailHandler struct {
	penjualan  *service.PenjualanService
	mitra      *service.MitraService
	gudang     *service.GudangService
	appSetting *service.AppSettingService
}

// NewPenjualanEmailHandler konstruktor.
func NewPenjualanEmailHandler(
	pj *service.PenjualanService,
	mr *service.MitraService,
	gr *service.GudangService,
	as *service.AppSettingService,
) *PenjualanEmailHandler {
	return &PenjualanEmailHandler{
		penjualan: pj, mitra: mr, gudang: gr, appSetting: as,
	}
}

// EmailKwitansi POST /penjualan/:id/email
// body: to (email), message (custom message)
func (h *PenjualanEmailHandler) EmailKwitansi(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	to := strings.TrimSpace(c.FormValue("to"))
	customMsg := strings.TrimSpace(c.FormValue("message"))
	if to == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "alamat email wajib diisi")
	}

	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}

	// Generate PDF.
	tokoInfo, _ := h.appSetting.TokoInfo(ctx)
	pdfBytes, err := pdf.GenerateKwitansiA5(pj, mitra, gudang, tokoInfo, "ASLI")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal generate PDF: "+err.Error())
	}

	// Load SMTP config.
	smtpCfg, err := h.appSetting.SMTPConfig(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if smtpCfg == nil || !smtpCfg.Enabled {
		return c.JSON(http.StatusUnprocessableEntity, echo.Map{
			"success":     false,
			"error":       "SMTP belum dikonfigurasi",
			"setup_url":   "/setting/smtp",
		})
	}

	tokoNama := "Toko"
	if tokoInfo != nil && tokoInfo.Nama != "" {
		tokoNama = tokoInfo.Nama
	} else if gudang != nil {
		tokoNama = gudang.Nama
	}

	subject := fmt.Sprintf("Kwitansi %s - %s", pj.NomorKwitansi, tokoNama)
	defaultMsg := fmt.Sprintf(
		"Halo %s,\n\nTerlampir kwitansi pembelian Anda di %s:\n"+
			"Nomor : %s\nTanggal : %s\nTotal : %s\n\nTerima kasih.",
		mitra.Nama, tokoNama, pj.NomorKwitansi,
		pj.Tanggal.Format("02 Jan 2006"), format.Rupiah(pj.Total))
	if customMsg != "" {
		defaultMsg = customMsg + "\n\n---\n" + defaultMsg
	}

	sender := email.NewSender(email.Config{
		Host:     smtpCfg.Host,
		Port:     smtpCfg.Port,
		Username: smtpCfg.Username,
		Password: smtpCfg.Password,
		From:     smtpCfg.From,
	})
	filename := "kwitansi-" + sanitizeFilename(pj.NomorKwitansi) + ".pdf"
	err = sender.Send(email.Message{
		To:       to,
		Subject:  subject,
		BodyText: defaultMsg,
		Attachments: []email.Attachment{
			{Filename: filename, ContentType: "application/pdf", Content: pdfBytes},
		},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	return c.JSON(http.StatusOK, echo.Map{"success": true, "message": "Email terkirim ke " + to})
}

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/format"
	"github.com/omanjaya/tokobangunan/internal/print/pdf"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// PenjualanShareHandler - WhatsApp share link + signed share PDF link.
type PenjualanShareHandler struct {
	penjualan  *service.PenjualanService
	mitra      *service.MitraService
	gudang     *service.GudangService
	appSetting *service.AppSettingService
	secret     []byte
}

// NewPenjualanShareHandler konstruktor.
func NewPenjualanShareHandler(
	pj *service.PenjualanService,
	mr *service.MitraService,
	gr *service.GudangService,
	as *service.AppSettingService,
	sessionSecret string,
) *PenjualanShareHandler {
	return &PenjualanShareHandler{
		penjualan: pj, mitra: mr, gudang: gr, appSetting: as,
		secret: []byte(sessionSecret),
	}
}

// WhatsAppLink GET /penjualan/:id/wa-link?to=<phone>
// Return JSON {url: "https://wa.me/..."}.
func (h *PenjualanShareHandler) WhatsAppLink(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
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

	// Resolve target phone: query 'to' atau dari mitra.kontak.
	rawPhone := strings.TrimSpace(c.QueryParam("to"))
	if rawPhone == "" && mitra.Kontak != nil {
		rawPhone = *mitra.Kontak
	}
	phone := normalizePhone(rawPhone)
	if phone == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nomor telepon tidak valid")
	}

	tokoInfo, _ := h.appSetting.TokoInfo(ctx)
	tokoNama := "Toko"
	if tokoInfo != nil && tokoInfo.Nama != "" {
		tokoNama = tokoInfo.Nama
	}

	// Generate signed PDF share link (24 jam).
	shareToken, err := auth.GenerateShareToken(h.secret,
		map[string]any{"id": pj.ID, "kind": "penjualan_pdf"},
		time.Now().Add(24*time.Hour))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal generate token")
	}
	publicURL := buildAbsoluteURL(c, "/share/penjualan?t="+shareToken)

	msg := fmt.Sprintf(
		"Halo %s,\n\nKwitansi pembelian Anda di %s:\nNo: %s\nTanggal: %s\nTotal: %s\nStatus: %s\n\nDetail kwitansi:\n%s\n\nTerima kasih.",
		mitra.Nama, tokoNama, pj.NomorKwitansi,
		pj.Tanggal.Format("02 Jan 2006"),
		format.Rupiah(pj.Total),
		string(pj.StatusBayar),
		publicURL)
	waURL := "https://wa.me/" + phone + "?text=" + url.QueryEscape(msg)
	return c.JSON(http.StatusOK, echo.Map{
		"url":        waURL,
		"public_url": publicURL,
	})
}

// GenerateShareLink POST /penjualan/:id/share-link?expires_hours=24
// Return JSON { url: "/share/penjualan?t=..." }
func (h *PenjualanShareHandler) GenerateShareLink(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	hrs, _ := strconv.Atoi(c.QueryParam("expires_hours"))
	if hrs <= 0 {
		hrs = 24
	}
	if hrs > 24*30 {
		hrs = 24 * 30
	}
	ctx := c.Request().Context()
	if _, err := h.penjualan.Get(ctx, id); err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	tok, err := auth.GenerateShareToken(h.secret,
		map[string]any{"id": id, "kind": "penjualan_pdf"},
		time.Now().Add(time.Duration(hrs)*time.Hour))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rel := "/share/penjualan?t=" + tok
	return c.JSON(http.StatusOK, echo.Map{
		"url":     buildAbsoluteURL(c, rel),
		"path":    rel,
		"expires": time.Now().Add(time.Duration(hrs) * time.Hour).Format(time.RFC3339),
	})
}

// SharePDF GET /share/penjualan?t=<token> - public PDF (no auth).
func (h *PenjualanShareHandler) SharePDF(c echo.Context) error {
	token := c.QueryParam("t")
	if token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token kosong")
	}
	var payload struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	if err := auth.VerifyShareToken(h.secret, token, &payload); err != nil {
		if errors.Is(err, auth.ErrShareTokenExpired) {
			return echo.NewHTTPError(http.StatusGone, "link kedaluwarsa")
		}
		return echo.NewHTTPError(http.StatusForbidden, "token tidak valid")
	}
	if payload.Kind != "penjualan_pdf" || payload.ID <= 0 {
		return echo.NewHTTPError(http.StatusForbidden, "token tidak valid")
	}

	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, payload.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
	}
	mitra, err := h.mitra.Get(ctx, pj.MitraID)
	if err != nil {
		return err
	}
	gudang, err := h.gudang.Get(ctx, pj.GudangID)
	if err != nil {
		return err
	}
	tokoInfo, _ := h.appSetting.TokoInfo(ctx)
	bytesPDF, err := pdf.GenerateKwitansiA5(pj, mitra, gudang, tokoInfo, "")
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`inline; filename="kwitansi-`+sanitizeFilename(pj.NomorKwitansi)+`.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", bytesPDF)
}

// normalizePhone - hapus +/-/spasi, prefix '62' kalau diawali '0'.
func normalizePhone(raw string) string {
	var sb strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	s := sb.String()
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "0") {
		s = "62" + strings.TrimPrefix(s, "0")
	}
	return s
}

// buildAbsoluteURL bangun URL absolut dari Echo context (scheme + host + path).
func buildAbsoluteURL(c echo.Context, rel string) string {
	scheme := "http"
	if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request().Host
	if h := c.Request().Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host + rel
}

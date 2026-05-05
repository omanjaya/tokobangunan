package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// ProdukFotoHandler - HTTP handler untuk upload/delete foto produk.
type ProdukFotoHandler struct {
	produk    *service.ProdukService
	uploadDir string // mis. "web/static/uploads/produk"
	urlPrefix string // mis. "/static/uploads/produk"
	maxSize   int64  // bytes
}

// NewProdukFotoHandler - default upload dir web/static/uploads/produk, max 2 MB.
func NewProdukFotoHandler(p *service.ProdukService) *ProdukFotoHandler {
	return &ProdukFotoHandler{
		produk:    p,
		uploadDir: "web/static/uploads/produk",
		urlPrefix: "/static/uploads/produk",
		maxSize:   2 * 1024 * 1024,
	}
}

// allowedMime memetakan MIME → ekstensi file.
var allowedMime = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// detectMime cek magic byte (signature) dari 12 byte pertama.
func detectMime(head []byte) string {
	if len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF {
		return "image/jpeg"
	}
	if len(head) >= 8 && head[0] == 0x89 && head[1] == 0x50 && head[2] == 0x4E && head[3] == 0x47 &&
		head[4] == 0x0D && head[5] == 0x0A && head[6] == 0x1A && head[7] == 0x0A {
		return "image/png"
	}
	// WEBP: "RIFF" .... "WEBP"
	if len(head) >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

// Upload POST /produk/:id/foto.
func (h *ProdukFotoHandler) Upload(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()

	p, err := h.produk.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}

	fh, err := c.FormFile("foto")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file foto wajib diisi")
	}
	if fh.Size > h.maxSize {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("ukuran file melebihi %d MB", h.maxSize/(1024*1024)))
	}

	src, err := fh.Open()
	if err != nil {
		return fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	head := make([]byte, 12)
	n, _ := io.ReadFull(src, head)
	mime := detectMime(head[:n])
	ext, ok := allowedMime[mime]
	if !ok {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType,
			"format tidak didukung, gunakan JPG/PNG/WEBP")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek upload: %w", err)
	}

	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		return fmt.Errorf("mkdir uploads: %w", err)
	}

	filename := fmt.Sprintf("%d_%s%s", id, uuid.NewString(), ext)
	dstPath := filepath.Join(h.uploadDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return fmt.Errorf("copy upload: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close upload: %w", err)
	}

	newURL := h.urlPrefix + "/" + filename

	// Hapus foto lama jika ada (best-effort).
	if p.FotoURL != nil {
		h.removeFile(*p.FotoURL)
	}

	if err := h.produk.SetFotoURL(ctx, id, &newURL); err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("set foto_url: %w", err)
	}

	redirect := fmt.Sprintf("/produk/%d/edit", id)
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", redirect)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, redirect)
}

// Delete POST /produk/:id/foto/delete.
func (h *ProdukFotoHandler) Delete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.produk.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrProdukNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "produk tidak ditemukan")
		}
		return err
	}
	if p.FotoURL != nil {
		h.removeFile(*p.FotoURL)
	}
	if err := h.produk.SetFotoURL(ctx, id, nil); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/produk/%d/edit", id)
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", redirect)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, redirect)
}

// removeFile hapus file disk dari URL relative (best-effort, ignore error).
func (h *ProdukFotoHandler) removeFile(url string) {
	// URL: /static/uploads/produk/<file> → web/static/uploads/produk/<file>
	if !strings.HasPrefix(url, h.urlPrefix+"/") {
		return
	}
	name := filepath.Base(url)
	if name == "" || name == "." || name == ".." {
		return
	}
	_ = os.Remove(filepath.Join(h.uploadDir, name))
}

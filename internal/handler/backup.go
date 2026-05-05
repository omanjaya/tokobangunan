// Package handler — backup.go: halaman admin untuk list backup files,
// download, trigger backup baru, dan restore. Owner-only (lihat wiring).
package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	"github.com/omanjaya/tokobangunan/internal/view/setting/backup"
)

// BackupHandler ekspose halaman & aksi backup.
type BackupHandler struct {
	dir         string // direktori backup, default ./backups
	scriptPath  string // path script backup
	restorePath string // path script restore
}

// NewBackupHandler — config dengan defaults.
func NewBackupHandler() *BackupHandler {
	return &BackupHandler{
		dir:         envOr("BACKUP_DIR", "./backups"),
		scriptPath:  envOr("BACKUP_SCRIPT", "scripts/backup.sh"),
		restorePath: envOr("RESTORE_SCRIPT", "scripts/restore.sh"),
	}
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// Index GET /setting/backup.
func (h *BackupHandler) Index(c echo.Context) error {
	files, err := h.listFiles()
	if err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	csrf, _ := c.Get("csrf").(string)
	props := backup.IndexProps{
		Nav:       layout.DefaultNav("/setting"),
		User:      userData(user),
		Files:     files,
		CSRFToken: csrf,
		Flash:     c.QueryParam("flash"),
		Error:     c.QueryParam("error"),
	}
	return RenderHTML(c, http.StatusOK, backup.Index(props))
}

// Trigger POST /setting/backup/run — exec backup.sh, blocking sederhana.
func (h *BackupHandler) Trigger(c echo.Context) error {
	cmd := exec.CommandContext(c.Request().Context(), "bash", h.scriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Backup gagal: "+truncate(string(out), 200)))
	}
	return c.Redirect(http.StatusSeeOther, "/setting/backup?flash=Backup+berhasil+dibuat")
}

// Download GET /setting/backup/:filename/download.
func (h *BackupHandler) Download(c echo.Context) error {
	name := safeBackupName(c.Param("filename"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nama file tidak valid")
	}
	full := filepath.Join(h.dir, name)
	if _, err := os.Stat(full); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "backup tidak ditemukan")
	}
	c.Response().Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, name))
	return c.File(full)
}

// Restore POST /setting/backup/:filename/restore — DESTRUCTIVE.
func (h *BackupHandler) Restore(c echo.Context) error {
	name := safeBackupName(c.Param("filename"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nama file tidak valid")
	}
	full := filepath.Join(h.dir, name)
	if _, err := os.Stat(full); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "backup tidak ditemukan")
	}
	cmd := exec.CommandContext(c.Request().Context(), "bash", h.restorePath, full)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Restore gagal: "+truncate(string(out), 200)))
	}
	return c.Redirect(http.StatusSeeOther, "/setting/backup?flash=Restore+berhasil")
}

// ----- helpers ---------------------------------------------------------------

func (h *BackupHandler) listFiles() ([]backup.FileInfo, error) {
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}
	out := make([]backup.FileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".dump") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, backup.FileInfo{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// safeBackupRe — whitelist karakter aman untuk nama backup file.
var safeBackupRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.dump$`)

// safeBackupName tolak path traversal & karakter aneh, hanya allow basename.
func safeBackupName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, "/\\") || strings.Contains(s, "..") {
		return ""
	}
	if !safeBackupRe.MatchString(s) {
		return ""
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

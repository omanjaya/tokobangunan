// Package handler — backup.go: halaman admin untuk list backup files,
// download, trigger backup baru, dan restore. Owner-only (lihat wiring).
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	"github.com/omanjaya/tokobangunan/internal/view/setting/backup"
)

// restorePhrase — string konfirmasi yang user wajib ketik untuk restore.
const restorePhrase = "RESTORE DATABASE"

// restoreLockFile — sentinel file penanda restore sedang berjalan.
const restoreLockFile = "tmp/restore.lock"

// restoreTimeout — batas atas eksekusi script restore.
const restoreTimeout = 10 * time.Minute

// restoreMu — mutex proses untuk single-instance restore.
var restoreMu sync.Mutex

// BackupHandler ekspose halaman & aksi backup.
type BackupHandler struct {
	dir         string // direktori backup, default ./backups
	scriptPath  string // path script backup
	restorePath string // path script restore
	authStore   *auth.Store
}

// NewBackupHandler — config dengan defaults.
func NewBackupHandler(authStore *auth.Store) *BackupHandler {
	return &BackupHandler{
		dir:         envOr("BACKUP_DIR", "./backups"),
		scriptPath:  envOr("BACKUP_SCRIPT", "scripts/backup.sh"),
		restorePath: envOr("RESTORE_SCRIPT", "scripts/restore.sh"),
		authStore:   authStore,
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
		Nav:           layout.DefaultNav("/setting"),
		User:          userData(user),
		Files:         files,
		CSRFToken:     csrf,
		Flash:         c.QueryParam("flash"),
		Error:         c.QueryParam("error"),
		RestorePhrase: restorePhrase,
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
//
// Multi-layer guard:
//  1. CSRF (echo middleware)
//  2. confirm_phrase wajib match konstanta restorePhrase
//  3. password_confirmation diverifikasi ulang via auth.VerifyPassword
//  4. lock file tmp/restore.lock + global mutex (single-instance)
//  5. timeout context 10 menit untuk script restore
func (h *BackupHandler) Restore(c echo.Context) error {
	ctx := c.Request().Context()
	cur := auth.CurrentUser(c)
	if cur == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	name := safeBackupName(c.Param("filename"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nama file tidak valid")
	}
	full := filepath.Join(h.dir, name)
	if _, err := os.Stat(full); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "backup tidak ditemukan")
	}

	phrase := strings.TrimSpace(c.FormValue("confirm_phrase"))
	password := c.FormValue("password_confirmation")

	if phrase != restorePhrase {
		slog.WarnContext(ctx, "restore denied: phrase mismatch",
			"user_id", cur.ID, "username", cur.Username, "file", name,
			"remote", c.Request().RemoteAddr)
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Frasa konfirmasi tidak sesuai."))
	}
	if password == "" {
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Password wajib diisi untuk restore."))
	}

	ok, verr := auth.VerifyPassword(password, cur.PasswordHash)
	if verr != nil || !ok {
		slog.WarnContext(ctx, "restore denied: password mismatch",
			"user_id", cur.ID, "username", cur.Username, "file", name,
			"remote", c.Request().RemoteAddr)
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Password salah."))
	}

	// Single-instance guard: mutex + lock file.
	if !restoreMu.TryLock() {
		return echo.NewHTTPError(http.StatusConflict, "restore sedang berjalan")
	}
	defer restoreMu.Unlock()

	if _, err := os.Stat(restoreLockFile); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "restore sedang berjalan")
	}
	if err := os.MkdirAll(filepath.Dir(restoreLockFile), 0o755); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	lf, err := os.OpenFile(restoreLockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return echo.NewHTTPError(http.StatusConflict, "restore sedang berjalan")
		}
		return fmt.Errorf("create lock: %w", err)
	}
	_ = lf.Close()
	defer func() { _ = os.Remove(restoreLockFile) }()

	slog.InfoContext(ctx, "restore start",
		"user_id", cur.ID, "username", cur.Username, "file", name)

	runCtx, cancel := context.WithTimeout(ctx, restoreTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", h.restorePath, full)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.ErrorContext(ctx, "restore failed",
			"user_id", cur.ID, "file", name, "error", err)
		return c.Redirect(http.StatusSeeOther,
			"/setting/backup?error="+url.QueryEscape("Restore gagal: "+truncate(string(out), 200)))
	}

	slog.InfoContext(ctx, "restore success",
		"user_id", cur.ID, "username", cur.Username, "file", name)
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
		if !isBackupFile(name) {
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

// isBackupFile — accept .dump, .dump.gz, .dump.gz.gpg.
func isBackupFile(name string) bool {
	return strings.HasSuffix(name, ".dump") ||
		strings.HasSuffix(name, ".dump.gz") ||
		strings.HasSuffix(name, ".dump.gz.gpg")
}

// safeBackupRe — whitelist karakter aman untuk nama backup file.
// Allow .dump, .dump.gz, .dump.gz.gpg.
var safeBackupRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.dump(\.gz(\.gpg)?)?$`)

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

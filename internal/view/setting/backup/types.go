// Package backup berisi view templ untuk halaman /setting/backup.
package backup

import (
	"fmt"
	"time"

	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// FileInfo - 1 entry backup file.
type FileInfo struct {
	Name    string
	Size    int64 // bytes
	ModTime time.Time
}

// IndexProps - props halaman list backup.
type IndexProps struct {
	Nav           layout.NavData
	User          layout.UserData
	Files         []FileInfo
	CSRFToken     string
	Flash         string
	Error         string
	RestorePhrase string // string yang user wajib ketik untuk konfirmasi restore
}

// SizeHuman format ukuran file.
func SizeHuman(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB"}[exp]
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), suffix)
}

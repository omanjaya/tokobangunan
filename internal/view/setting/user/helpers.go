// Package userview berisi templ component untuk modul setting/user.
package userview

import (
	"strconv"
	"time"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

func idToStr(id int64) string { return strconv.FormatInt(id, 10) }

// RoleBadgeVariant memetakan role → variant Badge.
//   owner=purple, admin=info(sky), kasir=success(emerald), gudang=warning(amber)
func RoleBadgeVariant(role string) string {
	switch role {
	case domain.RoleOwner:
		return "purple"
	case domain.RoleAdmin:
		return "info"
	case domain.RoleKasir:
		return "success"
	case domain.RoleGudang:
		return "warning"
	default:
		return "default"
	}
}

func RoleLabel(role string) string {
	switch role {
	case domain.RoleOwner:
		return "Owner"
	case domain.RoleAdmin:
		return "Admin"
	case domain.RoleKasir:
		return "Kasir"
	case domain.RoleGudang:
		return "Gudang"
	default:
		return role
	}
}

func StringOrDash(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

func TimeOrDash(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return t.Local().Format("02 Jan 2006 15:04")
}

func GudangNameByID(items []domain.Gudang, id *int64) string {
	if id == nil {
		return "Semua cabang"
	}
	for _, g := range items {
		if g.ID == *id {
			return g.Nama
		}
	}
	return "—"
}

// RolesAll - daftar role untuk dropdown.
type RoleOption struct {
	Value string
	Label string
}

func RolesAll() []RoleOption {
	return []RoleOption{
		{domain.RoleOwner, "Owner"},
		{domain.RoleAdmin, "Admin"},
		{domain.RoleKasir, "Kasir"},
		{domain.RoleGudang, "Gudang"},
	}
}

// IsSelectedGudang true bila ptr GudangID == id.
func IsSelectedGudang(ptr *int64, id int64) bool {
	return ptr != nil && *ptr == id
}

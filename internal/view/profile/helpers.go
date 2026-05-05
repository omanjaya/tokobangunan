// Package profile berisi templ component untuk halaman self-service profil.
package profile

import (
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

func roleLabel(role string) string {
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

func roleVariant(role string) string {
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

// initialFromName - dua huruf inisial untuk avatar fallback.
func initialFromName(a *domain.UserAccount) string {
	if a == nil {
		return "?"
	}
	parts := strings.Fields(strings.TrimSpace(a.NamaLengkap))
	switch len(parts) {
	case 0:
		if a.Username != "" {
			return strings.ToUpper(string([]rune(a.Username)[0]))
		}
		return "?"
	case 1:
		r := []rune(parts[0])
		if len(r) >= 2 {
			return strings.ToUpper(string(r[:2]))
		}
		return strings.ToUpper(string(r))
	default:
		return strings.ToUpper(string([]rune(parts[0])[0])) +
			strings.ToUpper(string([]rune(parts[len(parts)-1])[0]))
	}
}

func lastLoginText(a *domain.UserAccount) string {
	if a == nil || a.LastLoginAt == nil || a.LastLoginAt.IsZero() {
		return "—"
	}
	return a.LastLoginAt.Local().Format("02 Jan 2006 15:04")
}

func usernameOf(a *domain.UserAccount) string {
	if a == nil {
		return ""
	}
	return a.Username
}

func namaValue(p ShowProps) string {
	if p.Form.NamaLengkap != "" {
		return p.Form.NamaLengkap
	}
	if p.Account != nil {
		return p.Account.NamaLengkap
	}
	return ""
}

func emailValue(p ShowProps) string {
	if p.Form.Email != "" {
		return p.Form.Email
	}
	if p.Account != nil && p.Account.Email != nil {
		return *p.Account.Email
	}
	return ""
}

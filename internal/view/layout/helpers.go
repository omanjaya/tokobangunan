package layout

import (
	"context"

	"github.com/a-h/templ"

	appmw "github.com/omanjaya/tokobangunan/internal/middleware"
)

// CSPNonce returns the per-request CSP nonce from context. Use inside templ
// inline <script> tags as: <script nonce={ layout.CSPNonce(ctx) }>.
// Returns "" when no nonce is present (tests / direct render).
func CSPNonce(ctx context.Context) string {
	return appmw.CSPNonce(ctx)
}

// descOrDefault mengembalikan meta description dengan fallback generic
// bila string kosong. Dipakai di <head> AppShell / AuthLayout.
func descOrDefault(d string) string {
	if d == "" {
		return "Tokobangunan ERP — sistem manajemen toko bahan bangunan"
	}
	return d
}

// brandSlug mengembalikan slug pendek (2-3 char) untuk avatar logo.
func brandSlug(nav NavData) string {
	if nav.BrandSlug != "" {
		return nav.BrandSlug
	}
	if nav.BrandName == "" {
		return "TB"
	}
	if len(nav.BrandName) <= 2 {
		return nav.BrandName
	}
	return nav.BrandName[:2]
}

// logoutHref menentukan URL logout (default /logout).
func logoutHref(u UserData) templ.SafeURL {
	if u.LogoutHref != "" {
		return templ.SafeURL(u.LogoutHref)
	}
	return templ.SafeURL("/logout")
}

// profileHref menentukan URL halaman profile (default /profil).
func profileHref(u UserData) templ.SafeURL {
	if u.ProfileHref != "" {
		return templ.SafeURL(u.ProfileHref)
	}
	return templ.SafeURL("/profil")
}

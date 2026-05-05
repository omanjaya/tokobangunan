package layout

import "github.com/a-h/templ"

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

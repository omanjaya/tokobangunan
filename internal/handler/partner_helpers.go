package handler

import (
	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

// navMitra dipakai handler mitra untuk highlight item navigasi mitra.
func navMitra() layout.NavData {
	return layout.DefaultNav("/mitra")
}

// navSupplier dipakai handler supplier untuk highlight item supplier.
func navSupplier() layout.NavData {
	return layout.DefaultNav("/supplier")
}

// userData mapping auth.User → layout.UserData (yang dipakai topbar/sidebar).
func userData(u *auth.User) layout.UserData {
	if u == nil {
		return layout.UserData{}
	}
	return layout.UserData{
		Name: u.NamaLengkap,
		Role: u.Role,
	}
}

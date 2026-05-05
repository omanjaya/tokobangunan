// Package layout berisi komponen kerangka aplikasi: AppShell, Sidebar, TopBar,
// AuthLayout. Tidak mengandung business logic.
package layout

// BreadcrumbItem mewakili satu segmen breadcrumb di TopBar.
// Item terakhir dianggap halaman aktif (tidak di-link).
type BreadcrumbItem struct {
	Label string
	Href  string
}

// NavItem mewakili satu link sidebar.
// IconName adalah nama Lucide icon (lihat package icon).
// Active jika true akan diberi style highlight.
type NavItem struct {
	Label     string
	Href      string
	IconName  string
	Active    bool
	Badge     string // optional badge teks (mis. "3" untuk notifikasi)
	IsSection bool   // jika true, render sebagai header section, bukan link
}

// NavData adalah seluruh isi sidebar.
type NavData struct {
	Items      []NavItem
	BrandName  string // teks logo (sementara plaintext sebelum aset logo)
	BrandSlug  string // singkat untuk collapsed mode
	ActivePath string // path saat ini, dipakai handler untuk set Active
}

// UserData ditampilkan di TopBar dan Sidebar footer.
type UserData struct {
	Name        string
	Role        string
	GudangName  string // gudang aktif yang dipilih user
	AvatarInit  string // inisial untuk fallback avatar
	LogoutHref  string
	ProfileHref string
}

// AppShellProps adalah parameter untuk AppShell.
type AppShellProps struct {
	Title      string
	Nav        NavData
	User       UserData
	Breadcrumb []BreadcrumbItem
}

// AuthLayoutProps adalah parameter untuk AuthLayout (tanpa sidebar).
type AuthLayoutProps struct {
	Title string
}

// DefaultNav membangun navigasi default Fase 1.
// activePath = current request path (mis. "/penjualan").
func DefaultNav(activePath string) NavData {
	items := []NavItem{
		{Label: "Dashboard", Href: "/", IconName: "home"},

		{Label: "Transaksi", IsSection: true},
		{Label: "Penjualan", Href: "/penjualan", IconName: "shopping-cart"},
		{Label: "Pembelian", Href: "/pembelian", IconName: "shopping-bag"},
		{Label: "Mutasi", Href: "/mutasi", IconName: "truck"},
		{Label: "Stok Opname", Href: "/opname", IconName: "clipboard-check"},

		{Label: "Keuangan", IsSection: true},
		{Label: "Kas", Href: "/kas", IconName: "wallet"},
		{Label: "Piutang", Href: "/piutang", IconName: "clock"},
		{Label: "Pembayaran", Href: "/pembayaran", IconName: "credit-card"},

		{Label: "Inventory", IsSection: true},
		{Label: "Stok", Href: "/stok", IconName: "package"},
		{Label: "Produk", Href: "/produk", IconName: "boxes"},
		{Label: "Satuan", Href: "/satuan", IconName: "ruler"},

		{Label: "Mitra & Supplier", IsSection: true},
		{Label: "Mitra", Href: "/mitra", IconName: "users"},
		{Label: "Supplier", Href: "/supplier", IconName: "briefcase"},

		{Label: "Lainnya", IsSection: true},
		{Label: "Laporan", Href: "/laporan", IconName: "bar-chart-3"},
		{Label: "Setting", Href: "/setting", IconName: "settings"},
	}
	for i := range items {
		if items[i].IsSection {
			continue
		}
		if items[i].Href == activePath ||
			(items[i].Href != "/" && hasPrefix(activePath, items[i].Href)) {
			items[i].Active = true
		}
	}
	return NavData{
		Items:      items,
		BrandName:  "TOKOBANGUNAN",
		BrandSlug:  "TB",
		ActivePath: activePath,
	}
}

// hasPrefix tanpa import strings (kecil, cukup).
func hasPrefix(s, p string) bool {
	if len(p) > len(s) {
		return false
	}
	return s[:len(p)] == p
}

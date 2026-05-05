// Package components berisi UI komponen reusable: button, input, card, alert,
// badge, empty state, toast helper. Pure rendering, tidak ada business logic.
package components

// btnVariantClass memetakan variant button → class composer (memakai
// shorthand di app.css, bukan utility full).
func btnVariantClass(variant string) string {
	switch variant {
	case "primary":
		return "btn-primary"
	case "danger":
		return "btn-danger"
	case "ghost":
		return "btn-ghost"
	case "secondary":
		fallthrough
	default:
		return "btn-secondary"
	}
}

// btnSizeClass memetakan size button → class.
func btnSizeClass(size string) string {
	switch size {
	case "sm":
		return "btn-sm"
	case "lg":
		return "btn-lg"
	case "md":
		fallthrough
	default:
		return "btn-md"
	}
}

// BadgeColor mengembalikan tailwind class untuk badge sesuai variant.
// Public sehingga bisa dipakai komponen domain (StokBadge, RoleBadge, dll).
func BadgeColor(variant string) string {
	switch variant {
	case "success":
		return "bg-success-50 text-success-700 ring-1 ring-inset ring-success-100"
	case "warning":
		return "bg-warning-50 text-warning-700 ring-1 ring-inset ring-warning-100"
	case "danger":
		return "bg-danger-50 text-danger-700 ring-1 ring-inset ring-danger-100"
	case "info":
		return "bg-info-50 text-info-700 ring-1 ring-inset ring-info-100"
	case "brand":
		return "bg-brand-50 text-brand-700 ring-1 ring-inset ring-brand-100"
	case "purple":
		return "bg-violet-50 text-violet-700 ring-1 ring-inset ring-violet-200"
	case "default":
		fallthrough
	default:
		return "bg-slate-100 text-slate-700 ring-1 ring-inset ring-slate-200"
	}
}

// alertVariant memetakan variant → (containerClass, iconName, iconClass).
type alertStyle struct {
	Container string
	IconName  string
	IconClass string
	TitleCls  string
}

func alertVariantStyle(variant string) alertStyle {
	switch variant {
	case "success":
		return alertStyle{
			Container: "bg-success-50 border-success-100 text-success-900",
			IconName:  "check-circle-2",
			IconClass: "text-success-600",
			TitleCls:  "text-success-900",
		}
	case "warning":
		return alertStyle{
			Container: "bg-warning-50 border-warning-100 text-warning-900",
			IconName:  "alert-triangle",
			IconClass: "text-warning-600",
			TitleCls:  "text-warning-900",
		}
	case "danger":
		return alertStyle{
			Container: "bg-danger-50 border-danger-100 text-danger-900",
			IconName:  "x-circle",
			IconClass: "text-danger-600",
			TitleCls:  "text-danger-900",
		}
	case "info":
		fallthrough
	default:
		return alertStyle{
			Container: "bg-info-50 border-info-100 text-info-900",
			IconName:  "info",
			IconClass: "text-info-600",
			TitleCls:  "text-info-900",
		}
	}
}

// emptyStringFallback returns def if s == "".
func emptyStringFallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// FormatRupiah memformat cents (BIGINT) menjadi string "Rp 12.500".
// Contoh: 1250000 cents → "Rp 12.500".
func FormatRupiah(cents int64) string {
	rupiah := cents / 100
	negative := rupiah < 0
	if negative {
		rupiah = -rupiah
	}
	digits := []byte{}
	if rupiah == 0 {
		digits = []byte{'0'}
	}
	for rupiah > 0 {
		digits = append([]byte{byte('0' + rupiah%10)}, digits...)
		rupiah /= 10
	}
	// Sisipkan titik tiap 3 digit dari kanan.
	out := make([]byte, 0, len(digits)+len(digits)/3+3)
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	prefix := "Rp "
	if negative {
		prefix = "-Rp "
	}
	return prefix + string(out)
}

// FormatNumber memformat float dengan 2 desimal dan separator ribuan titik.
// Cocok untuk stok minimum, faktor konversi.
func FormatNumber(v float64) string {
	// Bulatkan ke 2 desimal.
	intPart := int64(v)
	frac := v - float64(intPart)
	if frac < 0 {
		frac = -frac
	}
	cents := int64(frac*100 + 0.5)
	if cents >= 100 {
		// rounding overflow ke int part.
		if intPart >= 0 {
			intPart++
		} else {
			intPart--
		}
		cents = 0
	}

	negative := intPart < 0
	if negative {
		intPart = -intPart
	}
	digits := []byte{}
	if intPart == 0 {
		digits = []byte{'0'}
	}
	for intPart > 0 {
		digits = append([]byte{byte('0' + intPart%10)}, digits...)
		intPart /= 10
	}
	out := make([]byte, 0, len(digits)+len(digits)/3+5)
	if negative {
		out = append(out, '-')
	}
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	if cents > 0 {
		out = append(out, ',')
		out = append(out, byte('0'+cents/10))
		out = append(out, byte('0'+cents%10))
	}
	return string(out)
}

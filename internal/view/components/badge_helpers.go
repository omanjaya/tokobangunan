package components

// badgeDotColor mengembalikan warna dot kecil di BadgeWithDot.
func badgeDotColor(variant string) string {
	switch variant {
	case "success":
		return "bg-emerald-500"
	case "warning":
		return "bg-amber-500"
	case "danger":
		return "bg-rose-500"
	case "info":
		return "bg-sky-500"
	case "purple":
		return "bg-violet-500"
	default:
		return "bg-slate-400"
	}
}

// DeltaColor untuk indikator naik/turun di StatCard.
// dir: "up" | "down" | "" (netral)
// good: true bila up adalah hal positif (mis. omset). false bila up = bad
// (mis. piutang overdue).
//
// A11y: Pakai shade -700 (bukan -600) supaya kontras text-xs (small text) di
// atas background putih lulus WCAG AA (>=4.5:1). text-emerald-600 ~3.4:1 fail.
// font-medium menambah ketebalan stroke supaya keterbacaan optimal.
func DeltaColor(dir string, good bool) string {
	switch dir {
	case "up":
		if good {
			return "text-emerald-700 font-medium"
		}
		return "text-rose-700 font-medium"
	case "down":
		if good {
			return "text-rose-700 font-medium"
		}
		return "text-emerald-700 font-medium"
	default:
		return "text-slate-600"
	}
}

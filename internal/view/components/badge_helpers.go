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
func DeltaColor(dir string, good bool) string {
	switch dir {
	case "up":
		if good {
			return "text-emerald-600"
		}
		return "text-rose-600"
	case "down":
		if good {
			return "text-rose-600"
		}
		return "text-emerald-600"
	default:
		return "text-slate-500"
	}
}

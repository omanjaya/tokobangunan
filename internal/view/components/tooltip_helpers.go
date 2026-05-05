package components

// tooltipPosClass mengembalikan kelas Tailwind untuk posisi tooltip.
// Default "top".
func tooltipPosClass(pos string) string {
	base := "absolute z-30 px-2 py-1 text-xs leading-tight whitespace-nowrap " +
		"rounded-md bg-slate-900 text-white shadow-lg pointer-events-none"
	switch pos {
	case "bottom":
		return base + " top-full left-1/2 -translate-x-1/2 mt-1"
	case "left":
		return base + " right-full top-1/2 -translate-y-1/2 mr-1"
	case "right":
		return base + " left-full top-1/2 -translate-y-1/2 ml-1"
	case "top":
		fallthrough
	default:
		return base + " bottom-full left-1/2 -translate-x-1/2 mb-1"
	}
}

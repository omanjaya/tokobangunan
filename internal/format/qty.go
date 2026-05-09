package format

import (
	"strconv"
	"strings"
)

// Qty format float ke string singkat: trim trailing zero + decimal point.
// 1.0000 -> "1", 0.5000 -> "0.5", 12.3450 -> "12.345".
func Qty(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

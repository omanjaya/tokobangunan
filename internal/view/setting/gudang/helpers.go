// Package gudangview berisi templ component untuk modul setting/gudang.
package gudangview

import "strconv"

func idToStr(id int64) string { return strconv.FormatInt(id, 10) }

func stringOrDash(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

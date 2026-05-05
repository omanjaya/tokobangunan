package components

import "net/url"

// urlQueryEscape membungkus url.QueryEscape supaya bisa dipanggil dari templ.
func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

package components

import "strings"

// jsEscape sangat sederhana untuk meng-escape string agar aman jadi literal
// di dalam single-quote JS Alpine x-init. Tidak boleh dipakai untuk untrusted
// input - di sini argument berasal dari handler kita sendiri.
func jsEscape(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		"\n", `\n`,
		"\r", `\r`,
		"<", `<`,
		">", `>`,
	)
	return r.Replace(s)
}

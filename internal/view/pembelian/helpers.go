package pembelian

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// formAction return URL action utk form: /pembelian (create) atau
// /pembelian/:id (update).
func formAction(p FormProps) templ.SafeURL {
	if p.EditID > 0 {
		return templ.SafeURL("/pembelian/" + strconv.FormatInt(p.EditID, 10))
	}
	return templ.SafeURL("/pembelian")
}

func formTitle(p FormProps) string {
	if p.EditID > 0 {
		return "Edit Pembelian"
	}
	return "Pembelian Baru"
}

func formSubtitle(p FormProps) string {
	if p.EditID > 0 {
		return "Revisi header & item pembelian. Stok lama akan di-rollback."
	}
	return "Catat pembelian dari supplier ke gudang."
}

func formCrumb(p FormProps) string {
	if p.EditID > 0 {
		return "Edit"
	}
	return "Baru"
}

// errOf ambil pesan error per field name (lowercase).
func errOf(p FormProps, field string) string {
	if p.Errors == nil {
		return ""
	}
	return p.Errors[field]
}

// itemRowCount jumlah baris item yang akan dirender (minimal 1).
func itemRowCount(p FormProps) int {
	if n := len(p.Input.Items); n > 0 {
		return n
	}
	return 1
}

func itemProdukIDStr(p FormProps, idx int) string {
	if idx < len(p.Input.Items) && p.Input.Items[idx].ProdukID > 0 {
		return strconv.FormatInt(p.Input.Items[idx].ProdukID, 10)
	}
	return ""
}

func itemQtyStr(p FormProps, idx int) string {
	if idx < len(p.Input.Items) && p.Input.Items[idx].Qty > 0 {
		return strconv.FormatFloat(p.Input.Items[idx].Qty, 'f', -1, 64)
	}
	return ""
}

func itemSatuanID(p FormProps, idx int) int64 {
	if idx < len(p.Input.Items) {
		return p.Input.Items[idx].SatuanID
	}
	return 0
}

func itemHargaStr(p FormProps, idx int) string {
	if idx < len(p.Input.Items) && p.Input.Items[idx].HargaSatuan > 0 {
		return strconv.FormatInt(p.Input.Items[idx].HargaSatuan, 10)
	}
	return ""
}

func int64ToStr(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// satuanOptionsScriptHTML render <script> yang set window.__satuanOptionsHTML
// jadi opsi <option> untuk satuan dropdown (dipakai JS addItemRow).
func satuanOptionsScriptHTML(p FormProps) string {
	var b strings.Builder
	for _, s := range p.Satuans {
		b.WriteString(`<option value="`)
		b.WriteString(strconv.FormatInt(s.ID, 10))
		b.WriteString(`">`)
		b.WriteString(escapeHTML(s.Kode))
		b.WriteString(`</option>`)
	}
	js := `<script>window.__satuanOptionsHTML = ` + jsString(b.String()) + `;</script>`
	return js
}

// jsString quote string untuk JS (escape backslash, quote, newline).
func jsString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '<':
			b.WriteString(`<`)
		case '>':
			b.WriteString(`>`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

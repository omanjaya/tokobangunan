// Package auditview - templ component untuk modul setting/audit-log.
package auditview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// FilterValues - nilai filter dari query string (raw, untuk re-populate form).
type FilterValues struct {
	Tabel    string
	Aksi     string
	UserID   string
	RecordID string
	From     string
	To       string
}

func IDToStr(id int64) string { return strconv.FormatInt(id, 10) }

// AksiBadgeVariant memetakan aksi → variant Badge.
func AksiBadgeVariant(aksi string) string {
	switch aksi {
	case domain.AuditAksiCreate:
		return "success"
	case domain.AuditAksiUpdate:
		return "info"
	case domain.AuditAksiDelete:
		return "danger"
	case domain.AuditAksiLogin, domain.AuditAksiLogout:
		return "default"
	default:
		return "default"
	}
}

// PrettyJSON - format payload jsonb ke string indent. Empty kalau nil.
func PrettyJSON(raw *json.RawMessage) string {
	if raw == nil || len(*raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, *raw, "", "  "); err != nil {
		return string(*raw)
	}
	return buf.String()
}

// UserLabel - tampilan user untuk row tabel.
func UserLabel(username, nama string) string {
	if username == "" && nama == "" {
		return "(sistem)"
	}
	if nama != "" && username != "" {
		return fmt.Sprintf("%s (%s)", nama, username)
	}
	if nama != "" {
		return nama
	}
	return username
}

// PageURL - rebuild query string dengan page baru.
func PageURL(f FilterValues, page int) string {
	v := url.Values{}
	if f.Tabel != "" {
		v.Set("tabel", f.Tabel)
	}
	if f.Aksi != "" {
		v.Set("aksi", f.Aksi)
	}
	if f.UserID != "" {
		v.Set("user_id", f.UserID)
	}
	if f.RecordID != "" {
		v.Set("record_id", f.RecordID)
	}
	if f.From != "" {
		v.Set("from", f.From)
	}
	if f.To != "" {
		v.Set("to", f.To)
	}
	v.Set("page", strconv.Itoa(page))
	return "/setting/audit-log?" + v.Encode()
}

// AksiOptions - opsi dropdown filter.
type Option struct {
	Value string
	Label string
}

func AksiOptions() []Option {
	out := make([]Option, 0, len(domain.AuditAksiList()))
	for _, a := range domain.AuditAksiList() {
		out = append(out, Option{Value: a, Label: a})
	}
	return out
}

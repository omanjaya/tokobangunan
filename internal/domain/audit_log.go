package domain

import (
	"encoding/json"
	"time"
)

// Aksi audit yang dikenali. Tidak strict (kolom TEXT) tapi pakai konstan
// supaya konsisten antar service.
const (
	AuditAksiCreate = "CREATE"
	AuditAksiUpdate = "UPDATE"
	AuditAksiDelete = "DELETE"
	AuditAksiLogin  = "LOGIN"
	AuditAksiLogout = "LOGOUT"
)

// AuditLog - satu entri di tabel audit_log. UserUsername & UserNama berasal
// dari JOIN ke tabel "user" (nullable kalau user dihapus).
type AuditLog struct {
	ID            int64
	UserID        *int64
	UserUsername  string
	UserNama      string
	Aksi          string
	Tabel         string
	RecordID      int64
	PayloadBefore *json.RawMessage
	PayloadAfter  *json.RawMessage
	IP            string
	UserAgent     string
	RequestID     *string
	CreatedAt     time.Time
}

// AuditAksiList - opsi dropdown filter aksi.
func AuditAksiList() []string {
	return []string{AuditAksiCreate, AuditAksiUpdate, AuditAksiDelete, AuditAksiLogin, AuditAksiLogout}
}

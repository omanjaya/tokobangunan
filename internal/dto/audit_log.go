package dto

import "time"

// AuditLogFilterInput - parameter form filter list audit log.
// Field optional pakai pointer agar empty != filter aktif.
type AuditLogFilterInput struct {
	Tabel    string
	Aksi     string
	UserID   *int64
	RecordID *int64
	From     *time.Time
	To       *time.Time
	Page     int
	PerPage  int
}

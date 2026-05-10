package service

import (
	"context"
	"encoding/json"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// AuditLogService - read & record entri audit. Untuk Fase 1, Record belum
// di-wire otomatis ke service mutasi data lain — service yang membutuhkan
// audit dapat memanggilnya secara eksplisit.
type AuditLogService struct {
	repo *repo.AuditLogRepo
}

func NewAuditLogService(r *repo.AuditLogRepo) *AuditLogService {
	return &AuditLogService{repo: r}
}

// RecordEntry - parameter Record. Pakai struct supaya pemanggil eksplisit
// menyebut field tanpa argumen positional yang panjang.
//
// IP/UserAgent/RequestID kosong → service akan fallback ke AuditMeta dari ctx
// (di-inject middleware appmw.AuditLog). Pemanggil yang punya konteks Echo
// (mis. handler/auth.go saat login_failed dgn UserID=0) bisa set manual.
type RecordEntry struct {
	UserID    *int64
	Aksi      string
	Tabel     string
	RecordID  int64
	Before    any
	After     any
	IP        string
	UserAgent string
	RequestID string
}

// Record - buat entri audit baru. Before/After di-marshal ke JSON;
// nil pointer atau nil interface dilewati.
//
// IP/UserAgent/RequestID/UserID di-auto-populate dari AuditMeta ctx kalau
// pemanggil tidak set eksplisit. Best-effort: ctx tanpa meta → field kosong/nil.
func (s *AuditLogService) Record(ctx context.Context, in RecordEntry) error {
	meta := auditMetaFromContext(ctx)
	if in.IP == "" {
		in.IP = meta.IP
	}
	if in.UserAgent == "" {
		in.UserAgent = meta.UserAgent
	}
	if in.RequestID == "" {
		in.RequestID = meta.RequestID
	}
	if in.UserID == nil && meta.UserID > 0 {
		uid := meta.UserID
		in.UserID = &uid
	}
	var reqID *string
	if in.RequestID != "" {
		rid := in.RequestID
		reqID = &rid
	}
	l := &domain.AuditLog{
		UserID:    in.UserID,
		Aksi:      in.Aksi,
		Tabel:     in.Tabel,
		RecordID:  in.RecordID,
		IP:        in.IP,
		UserAgent: in.UserAgent,
		RequestID: reqID,
	}
	if in.Before != nil {
		b, err := json.Marshal(in.Before)
		if err == nil {
			rm := json.RawMessage(b)
			l.PayloadBefore = &rm
		}
	}
	if in.After != nil {
		b, err := json.Marshal(in.After)
		if err == nil {
			rm := json.RawMessage(b)
			l.PayloadAfter = &rm
		}
	}
	return s.repo.Create(ctx, l)
}

func (s *AuditLogService) List(ctx context.Context, f repo.ListAuditFilter) (PageResult[domain.AuditLog], error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return PageResult[domain.AuditLog]{}, err
	}
	return NewPageResult(items, total, f.Page, f.PerPage), nil
}

func (s *AuditLogService) Get(ctx context.Context, id int64) (*domain.AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AuditLogService) ListTabel(ctx context.Context) ([]string, error) {
	return s.repo.ListTabel(ctx)
}

// auditUserKey adalah context key untuk membawa user_id dari handler ke service
// supaya method yang signature-nya tidak menerima userID (mis. MitraService.Update)
// tetap bisa menulis audit. Best-effort: jika tidak ada → audit user_id = nil.
type auditUserKey struct{}

// WithAuditUser attach user_id ke ctx untuk dikonsumsi oleh service-level audit.
func WithAuditUser(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, auditUserKey{}, userID)
}

// AuditUserFromContext extract user_id (0 = absent).
func AuditUserFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(auditUserKey{}).(int64)
	if v > 0 {
		return v
	}
	// Fallback: kalau caller pakai WithAuditMeta saja (tanpa WithAuditUser),
	// ambil UserID dari meta.
	return auditMetaFromContext(ctx).UserID
}

// AuditMeta - metadata request yang di-inject middleware ke ctx supaya
// service-level audit calls bisa otomatis isi IP/UserAgent/RequestID tanpa
// harus thread Echo context melalui semua signature service.
type AuditMeta struct {
	UserID    int64
	IP        string
	UserAgent string
	RequestID string
}

type ctxAuditMetaKey struct{}

// WithAuditMeta attach AuditMeta ke ctx. Dipanggil dari middleware sebelum
// handler/service eksekusi.
func WithAuditMeta(ctx context.Context, m AuditMeta) context.Context {
	return context.WithValue(ctx, ctxAuditMetaKey{}, m)
}

// auditMetaFromContext - extract meta (zero-value AuditMeta kalau absent).
func auditMetaFromContext(ctx context.Context) AuditMeta {
	if v := ctx.Value(ctxAuditMetaKey{}); v != nil {
		if m, ok := v.(AuditMeta); ok {
			return m
		}
	}
	return AuditMeta{}
}

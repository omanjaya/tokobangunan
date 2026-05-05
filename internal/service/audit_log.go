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
type RecordEntry struct {
	UserID    *int64
	Aksi      string
	Tabel     string
	RecordID  int64
	Before    any
	After     any
	IP        string
	UserAgent string
}

// Record - buat entri audit baru. Before/After di-marshal ke JSON;
// nil pointer atau nil interface dilewati.
func (s *AuditLogService) Record(ctx context.Context, in RecordEntry) error {
	l := &domain.AuditLog{
		UserID:    in.UserID,
		Aksi:      in.Aksi,
		Tabel:     in.Tabel,
		RecordID:  in.RecordID,
		IP:        in.IP,
		UserAgent: in.UserAgent,
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
	return v
}

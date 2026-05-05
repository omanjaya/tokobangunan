package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListAuditFilter - parameter listing audit_log.
type ListAuditFilter struct {
	Tabel    *string
	Aksi     *string
	UserID   *int64
	RecordID *int64
	From     *time.Time
	To       *time.Time
	Page     int
	PerPage  int
}

func (f *ListAuditFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 50
	}
	if f.PerPage > 200 {
		f.PerPage = 200
	}
}

// AuditLogRepo akses tabel audit_log.
type AuditLogRepo struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepo(pool *pgxpool.Pool) *AuditLogRepo {
	return &AuditLogRepo{pool: pool}
}

const auditSelectColumns = `a.id, a.user_id, COALESCE(u.username, '') AS uname,
	COALESCE(u.nama_lengkap, '') AS unama,
	a.aksi, a.tabel, a.record_id, a.payload_before, a.payload_after,
	COALESCE(host(a.ip), '') AS ip, COALESCE(a.user_agent, '') AS ua, a.created_at`

func scanAuditLog(row pgx.Row, l *domain.AuditLog) error {
	return row.Scan(&l.ID, &l.UserID, &l.UserUsername, &l.UserNama,
		&l.Aksi, &l.Tabel, &l.RecordID, &l.PayloadBefore, &l.PayloadAfter,
		&l.IP, &l.UserAgent, &l.CreatedAt)
}

// Create insert log baru. Dipakai service.AuditLogService.Record. Untuk Fase 1
// belum auto-wired ke handler lain.
func (r *AuditLogRepo) Create(ctx context.Context, l *domain.AuditLog) error {
	const sql = `
		INSERT INTO audit_log (user_id, aksi, tabel, record_id, payload_before, payload_after, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, $8)
		RETURNING id, created_at`
	var before, after []byte
	if l.PayloadBefore != nil {
		before = []byte(*l.PayloadBefore)
	}
	if l.PayloadAfter != nil {
		after = []byte(*l.PayloadAfter)
	}
	row := r.pool.QueryRow(ctx, sql,
		l.UserID, l.Aksi, l.Tabel, l.RecordID,
		nullableJSON(before), nullableJSON(after),
		l.IP, nullableString(l.UserAgent),
	)
	if err := row.Scan(&l.ID, &l.CreatedAt); err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// List query log dengan filter + pagination. Total adalah count tanpa limit.
func (r *AuditLogRepo) List(ctx context.Context, f ListAuditFilter) ([]domain.AuditLog, int, error) {
	f.Normalize()

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if f.Tabel != nil && strings.TrimSpace(*f.Tabel) != "" {
		where = append(where, fmt.Sprintf("a.tabel = $%d", idx))
		args = append(args, *f.Tabel)
		idx++
	}
	if f.Aksi != nil && strings.TrimSpace(*f.Aksi) != "" {
		where = append(where, fmt.Sprintf("a.aksi = $%d", idx))
		args = append(args, *f.Aksi)
		idx++
	}
	if f.UserID != nil {
		where = append(where, fmt.Sprintf("a.user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.RecordID != nil {
		where = append(where, fmt.Sprintf("a.record_id = $%d", idx))
		args = append(args, *f.RecordID)
		idx++
	}
	if f.From != nil {
		where = append(where, fmt.Sprintf("a.created_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("a.created_at < $%d", idx))
		args = append(args, *f.To)
		idx++
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := `SELECT COUNT(*) FROM audit_log a WHERE ` + whereSQL
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit log: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT %s
		FROM audit_log a
		LEFT JOIN "user" u ON u.id = a.user_id
		WHERE %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d`,
		auditSelectColumns, whereSQL, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AuditLog, 0, f.PerPage)
	for rows.Next() {
		var l domain.AuditLog
		if err := scanAuditLog(rows, &l); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter audit log: %w", err)
	}
	return out, total, nil
}

// GetByID - ambil entri per ID.
func (r *AuditLogRepo) GetByID(ctx context.Context, id int64) (*domain.AuditLog, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM audit_log a
		LEFT JOIN "user" u ON u.id = a.user_id
		WHERE a.id = $1`, auditSelectColumns)
	row := r.pool.QueryRow(ctx, sql, id)
	var l domain.AuditLog
	if err := scanAuditLog(row, &l); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAuditLogNotFound
		}
		return nil, fmt.Errorf("get audit log: %w", err)
	}
	return &l, nil
}

// ListTabel - daftar nama tabel unik (untuk dropdown filter).
func (r *AuditLogRepo) ListTabel(ctx context.Context) ([]string, error) {
	const sql = `SELECT DISTINCT tabel FROM audit_log ORDER BY tabel ASC`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list audit tabel: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ErrAuditLogNotFound - sentinel error.
var ErrAuditLogNotFound = errors.New("audit log tidak ditemukan")

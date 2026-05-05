package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListUserFilter - parameter listing user.
type ListUserFilter struct {
	Query    string
	Role     *string
	GudangID *int64
	IsActive *bool
	Page     int
	PerPage  int
}

func (f *ListUserFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
}

// UserAccountRepo akses tabel "user" untuk modul management.
type UserAccountRepo struct {
	pool *pgxpool.Pool
}

func NewUserAccountRepo(pool *pgxpool.Pool) *UserAccountRepo {
	return &UserAccountRepo{pool: pool}
}

const userAccountColumns = `id, username, nama_lengkap, email, role, gudang_id,
	is_active, last_login_at, created_at, updated_at`

func scanUserAccount(row pgx.Row, u *domain.UserAccount) error {
	return row.Scan(&u.ID, &u.Username, &u.NamaLengkap, &u.Email, &u.Role,
		&u.GudangID, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserAccountRepo) List(ctx context.Context, f ListUserFilter) ([]domain.UserAccount, int, error) {
	f.Normalize()

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, fmt.Sprintf("(username ILIKE $%d OR nama_lengkap ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if f.Role != nil && strings.TrimSpace(*f.Role) != "" {
		where = append(where, fmt.Sprintf("role = $%d", idx))
		args = append(args, *f.Role)
		idx++
	}
	if f.GudangID != nil {
		where = append(where, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *f.GudangID)
		idx++
	}
	if f.IsActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *f.IsActive)
		idx++
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := `SELECT COUNT(*) FROM "user" WHERE ` + whereSQL
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM "user" WHERE %s ORDER BY username ASC LIMIT $%d OFFSET $%d`,
		userAccountColumns, whereSQL, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query user: %w", err)
	}
	defer rows.Close()

	out := make([]domain.UserAccount, 0, f.PerPage)
	for rows.Next() {
		var u domain.UserAccount
		if err := scanUserAccount(rows, &u); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter user: %w", err)
	}
	return out, total, nil
}

func (r *UserAccountRepo) GetByID(ctx context.Context, id int64) (*domain.UserAccount, error) {
	const sql = `SELECT ` + userAccountColumns + ` FROM "user" WHERE id = $1`
	row := r.pool.QueryRow(ctx, sql, id)
	var u domain.UserAccount
	if err := scanUserAccount(row, &u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserAccountNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *UserAccountRepo) GetByUsername(ctx context.Context, username string) (*domain.UserAccount, error) {
	const sql = `SELECT ` + userAccountColumns + ` FROM "user" WHERE username = $1`
	row := r.pool.QueryRow(ctx, sql, username)
	var u domain.UserAccount
	if err := scanUserAccount(row, &u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserAccountNotFound
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}

// Create insert user dengan password hash sudah di-precompute oleh service.
func (r *UserAccountRepo) Create(ctx context.Context, u *domain.UserAccount, passwordHash string) error {
	const sql = `
		INSERT INTO "user" (username, password_hash, nama_lengkap, email, role, gudang_id, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, sql,
		u.Username, passwordHash, u.NamaLengkap, u.Email, u.Role, u.GudangID, u.IsActive)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// Update tidak menyentuh password_hash, failed_attempts, locked_until.
func (r *UserAccountRepo) Update(ctx context.Context, u *domain.UserAccount) error {
	const sql = `
		UPDATE "user" SET
			username = $2, nama_lengkap = $3, email = $4, role = $5,
			gudang_id = $6, is_active = $7
		WHERE id = $1
		RETURNING updated_at`
	row := r.pool.QueryRow(ctx, sql,
		u.ID, u.Username, u.NamaLengkap, u.Email, u.Role, u.GudangID, u.IsActive)
	if err := row.Scan(&u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserAccountNotFound
		}
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// UpdatePassword set hash baru, reset failed_attempts/locked_until.
func (r *UserAccountRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	const sql = `
		UPDATE "user"
		SET password_hash = $2, failed_attempts = 0, locked_until = NULL
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserAccountNotFound
	}
	return nil
}

func (r *UserAccountRepo) SetActive(ctx context.Context, id int64, active bool) error {
	const sql = `UPDATE "user" SET is_active = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, active)
	if err != nil {
		return fmt.Errorf("set active user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserAccountNotFound
	}
	return nil
}

// GetPasswordHash dipakai service.ChangePassword untuk verify old password.
func (r *UserAccountRepo) GetPasswordHash(ctx context.Context, id int64) (string, error) {
	const sql = `SELECT password_hash FROM "user" WHERE id = $1`
	var hash string
	if err := r.pool.QueryRow(ctx, sql, id).Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrUserAccountNotFound
		}
		return "", fmt.Errorf("get password hash: %w", err)
	}
	return hash, nil
}

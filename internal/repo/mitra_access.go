package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// MitraAccessRepo CRUD raw pgx untuk tabel mitra_access_token.
type MitraAccessRepo struct {
	pool *pgxpool.Pool
}

// NewMitraAccessRepo konstruktor.
func NewMitraAccessRepo(pool *pgxpool.Pool) *MitraAccessRepo {
	return &MitraAccessRepo{pool: pool}
}

// Create insert token baru.
func (r *MitraAccessRepo) Create(ctx context.Context, t *domain.MitraAccessToken) error {
	const sql = `INSERT INTO mitra_access_token (mitra_id, token, expires_at)
		VALUES ($1, $2, $3) RETURNING id, created_at`
	if err := r.pool.QueryRow(ctx, sql, t.MitraID, t.Token, t.ExpiresAt).
		Scan(&t.ID, &t.CreatedAt); err != nil {
		return fmt.Errorf("create access token: %w", err)
	}
	return nil
}

// GetByToken lookup token aktif (belum revoked). Caller cek expiry.
func (r *MitraAccessRepo) GetByToken(ctx context.Context, token string) (*domain.MitraAccessToken, error) {
	const sql = `SELECT id, mitra_id, token, expires_at, created_at, revoked
		FROM mitra_access_token WHERE token = $1`
	var t domain.MitraAccessToken
	err := r.pool.QueryRow(ctx, sql, token).Scan(
		&t.ID, &t.MitraID, &t.Token, &t.ExpiresAt, &t.CreatedAt, &t.Revoked,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccessTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	return &t, nil
}

// Revoke set revoked=true untuk token id ini (milik mitraID).
func (r *MitraAccessRepo) Revoke(ctx context.Context, id, mitraID int64) error {
	const sql = `UPDATE mitra_access_token SET revoked = TRUE
		WHERE id = $1 AND mitra_id = $2`
	tag, err := r.pool.Exec(ctx, sql, id, mitraID)
	if err != nil {
		return fmt.Errorf("revoke access token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAccessTokenNotFound
	}
	return nil
}

// ListByMitra ambil semua token milik mitraID, order created_at desc.
func (r *MitraAccessRepo) ListByMitra(ctx context.Context, mitraID int64) ([]domain.MitraAccessToken, error) {
	const sql = `SELECT id, mitra_id, token, expires_at, created_at, revoked
		FROM mitra_access_token WHERE mitra_id = $1
		ORDER BY created_at DESC LIMIT 50`
	rows, err := r.pool.Query(ctx, sql, mitraID)
	if err != nil {
		return nil, fmt.Errorf("list access token: %w", err)
	}
	defer rows.Close()
	out := make([]domain.MitraAccessToken, 0, 8)
	for rows.Next() {
		var t domain.MitraAccessToken
		if err := rows.Scan(&t.ID, &t.MitraID, &t.Token, &t.ExpiresAt,
			&t.CreatedAt, &t.Revoked); err != nil {
			return nil, fmt.Errorf("scan access token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteExpired hapus token expired/revoked yang sudah > 30 hari.
func (r *MitraAccessRepo) DeleteExpired(ctx context.Context) error {
	const sql = `DELETE FROM mitra_access_token
		WHERE (revoked = TRUE OR expires_at < $1)`
	cutoff := time.Now().AddDate(0, 0, -30)
	_, err := r.pool.Exec(ctx, sql, cutoff)
	return err
}

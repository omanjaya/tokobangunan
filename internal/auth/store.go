package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SessionDuration  = 8 * time.Hour
	MaxFailedLogins  = 5
	LockoutDuration  = 30 * time.Minute
)

var (
	ErrUserNotFound      = errors.New("user tidak ditemukan")
	ErrUserLocked        = errors.New("akun dikunci sementara")
	ErrInvalidCredential = errors.New("username atau password salah")
	ErrSessionExpired    = errors.New("sesi sudah berakhir")
)

type User struct {
	ID            int64
	Username      string
	PasswordHash  string
	NamaLengkap   string
	Email         *string
	Role          string
	GudangID      *int64
	IsActive      bool
	LockedUntil   *time.Time
	FailedAttempts int
}

type Session struct {
	ID        uuid.UUID
	UserID    int64
	ExpiresAt time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	const q = `
		SELECT id, username, password_hash, nama_lengkap, email, role, gudang_id,
		       is_active, locked_until, failed_attempts
		FROM "user"
		WHERE username = $1
	`
	row := s.pool.QueryRow(ctx, q, username)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NamaLengkap, &u.Email,
		&u.Role, &u.GudangID, &u.IsActive, &u.LockedUntil, &u.FailedAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	const q = `
		SELECT id, username, password_hash, nama_lengkap, email, role, gudang_id,
		       is_active, locked_until, failed_attempts
		FROM "user"
		WHERE id = $1
	`
	row := s.pool.QueryRow(ctx, q, id)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NamaLengkap, &u.Email,
		&u.Role, &u.GudangID, &u.IsActive, &u.LockedUntil, &u.FailedAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

func (s *Store) IncrementFailedAttempts(ctx context.Context, userID int64) error {
	const q = `
		UPDATE "user"
		SET failed_attempts = failed_attempts + 1,
		    locked_until = CASE
		        WHEN failed_attempts + 1 >= $2 THEN now() + ($3::interval)
		        ELSE locked_until
		    END
		WHERE id = $1
	`
	lockInterval := fmt.Sprintf("%d minutes", int(LockoutDuration.Minutes()))
	_, err := s.pool.Exec(ctx, q, userID, MaxFailedLogins, lockInterval)
	if err != nil {
		return fmt.Errorf("increment failed: %w", err)
	}
	return nil
}

func (s *Store) ResetFailedAttempts(ctx context.Context, userID int64) error {
	const q = `
		UPDATE "user"
		SET failed_attempts = 0, locked_until = NULL, last_login_at = now()
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, ip net.IP, userAgent string) (*Session, error) {
	const q = `
		INSERT INTO session (user_id, ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, expires_at
	`
	expiresAt := time.Now().Add(SessionDuration)
	row := s.pool.QueryRow(ctx, q, userID, ip, userAgent, expiresAt)
	var sess Session
	if err := row.Scan(&sess.ID, &sess.ExpiresAt); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	sess.UserID = userID
	return &sess, nil
}

func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (*Session, error) {
	const q = `
		SELECT id, user_id, expires_at
		FROM session
		WHERE id = $1 AND expires_at > now()
	`
	row := s.pool.QueryRow(ctx, q, id)
	var sess Session
	err := row.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionExpired
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &sess, nil
}

func (s *Store) RefreshSession(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE session SET expires_at = $2 WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, time.Now().Add(SessionDuration))
	return err
}

func (s *Store) DeleteSession(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM session WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id)
	return err
}

// DeleteSessionsByUser hapus semua session milik user. Dipakai sesudah
// password change agar device lain ter-logout (mitigasi credential reuse).
func (s *Store) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	const q = `DELETE FROM session WHERE user_id = $1`
	_, err := s.pool.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user: %w", err)
	}
	return nil
}

// DeleteSessionsByUserExcept hapus semua session user kecuali sesi keep.
// Dipakai pada flow ChangePassword untuk pertahankan sesi yang sedang aktif.
func (s *Store) DeleteSessionsByUserExcept(ctx context.Context, userID int64, keep uuid.UUID) error {
	const q = `DELETE FROM session WHERE user_id = $1 AND id <> $2`
	_, err := s.pool.Exec(ctx, q, userID, keep)
	if err != nil {
		return fmt.Errorf("delete sessions except: %w", err)
	}
	return nil
}

// Authenticate verify username + password, handle locking. Mengembalikan user
// kalau valid, atau ErrInvalidCredential / ErrUserLocked.
func (s *Store) Authenticate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.GetUserByUsername(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		// Tetap proses argon2 dummy untuk timing-safe (mitigasi user enumeration).
		_, _ = VerifyPassword(password, dummyHash)
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInvalidCredential
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrUserLocked
	}

	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		_ = s.IncrementFailedAttempts(ctx, user.ID)
		return nil, ErrInvalidCredential
	}

	if err := s.ResetFailedAttempts(ctx, user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

// dummyHash dipakai untuk timing-safe verification ketika username tidak ada.
// Hash dari string acak — tidak akan match password asli apa pun.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$YWJjZGVmZ2hpamtsbW5vcA$Z2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU2Nzg5YWJjZGVm"

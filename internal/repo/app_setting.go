package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// AppSettingRepo akses tabel app_setting.
type AppSettingRepo struct {
	pool *pgxpool.Pool
}

func NewAppSettingRepo(pool *pgxpool.Pool) *AppSettingRepo {
	return &AppSettingRepo{pool: pool}
}

// ErrSettingNotFound - sentinel.
var ErrSettingNotFound = errors.New("setting tidak ditemukan")

// Get ambil 1 row by key. Return ErrSettingNotFound bila tidak ada.
func (r *AppSettingRepo) Get(ctx context.Context, key string) (*domain.AppSetting, error) {
	const sql = `SELECT id, key, value, updated_at, updated_by
		FROM app_setting WHERE key = $1`
	var s domain.AppSetting
	if err := r.pool.QueryRow(ctx, sql, key).Scan(
		&s.ID, &s.Key, &s.Value, &s.UpdatedAt, &s.UpdatedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSettingNotFound
		}
		return nil, fmt.Errorf("get setting: %w", err)
	}
	return &s, nil
}

// Set UPSERT key-value.
func (r *AppSettingRepo) Set(ctx context.Context, key string, value json.RawMessage, userID *int64) error {
	const sql = `INSERT INTO app_setting (key, value, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE
		  SET value = EXCLUDED.value,
		      updated_at = now(),
		      updated_by = EXCLUDED.updated_by`
	if _, err := r.pool.Exec(ctx, sql, key, value, userID); err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

// GetTokoInfo unmarshal key=toko_info ke TokoInfo. Return zero kalau belum ada.
func (r *AppSettingRepo) GetTokoInfo(ctx context.Context) (*domain.TokoInfo, error) {
	s, err := r.Get(ctx, domain.SettingKeyTokoInfo)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &domain.TokoInfo{}, nil
		}
		return nil, err
	}
	var t domain.TokoInfo
	if err := json.Unmarshal(s.Value, &t); err != nil {
		return nil, fmt.Errorf("unmarshal toko_info: %w", err)
	}
	return &t, nil
}

// GetPajakConfig unmarshal key=pajak_config. Return default kalau belum di-set.
func (r *AppSettingRepo) GetPajakConfig(ctx context.Context) (*domain.PajakConfig, error) {
	s, err := r.Get(ctx, domain.SettingKeyPajakConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &domain.PajakConfig{PPNPersen: 11.0}, nil
		}
		return nil, err
	}
	cfg := domain.PajakConfig{PPNPersen: 11.0}
	if err := json.Unmarshal(s.Value, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal pajak_config: %w", err)
	}
	return &cfg, nil
}

// GetSMTPConfig unmarshal key=smtp_config. Return zero kalau belum ada.
func (r *AppSettingRepo) GetSMTPConfig(ctx context.Context) (*domain.SMTPConfig, error) {
	s, err := r.Get(ctx, domain.SettingKeySMTPConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &domain.SMTPConfig{}, nil
		}
		return nil, err
	}
	var cfg domain.SMTPConfig
	if err := json.Unmarshal(s.Value, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal smtp_config: %w", err)
	}
	return &cfg, nil
}

// IsOnboardingDone cek key=onboarding_done.
func (r *AppSettingRepo) IsOnboardingDone(ctx context.Context) (bool, error) {
	s, err := r.Get(ctx, domain.SettingKeyOnboardingDone)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return false, nil
		}
		return false, err
	}
	var v bool
	if err := json.Unmarshal(s.Value, &v); err != nil {
		return false, fmt.Errorf("unmarshal onboarding_done: %w", err)
	}
	return v, nil
}

// RecentPembayaran - n pembayaran terbaru, dengan nama mitra.
type RecentPembayaranRow struct {
	ID         int64
	Tanggal    time.Time
	MitraNama  string
	Jumlah     int64
	Metode     string
	NomorRef   string
}

// RecentMutasiRow - n mutasi terbaru (ringkas).
type RecentMutasiRow struct {
	ID           int64
	Tanggal      time.Time
	NomorMutasi  string
	GudangAsal   string
	GudangTujuan string
	Status       string
}

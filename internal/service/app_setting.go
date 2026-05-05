package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/crypto"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// AppSettingService - layer business untuk app_setting key-value.
type AppSettingService struct {
	repo *repo.AppSettingRepo
}

func NewAppSettingService(r *repo.AppSettingRepo) *AppSettingService {
	return &AppSettingService{repo: r}
}

// TokoInfo ambil dari setting; nil-safe (empty struct kalau belum di-set).
func (s *AppSettingService) TokoInfo(ctx context.Context) (*domain.TokoInfo, error) {
	return s.repo.GetTokoInfo(ctx)
}

// UpdateTokoInfo simpan info toko ke key=toko_info.
func (s *AppSettingService) UpdateTokoInfo(
	ctx context.Context, in *domain.TokoInfo, userID *int64,
) error {
	if in == nil {
		return fmt.Errorf("toko info nil")
	}
	clean := domain.TokoInfo{
		Nama:        strings.TrimSpace(in.Nama),
		Alamat:      strings.TrimSpace(in.Alamat),
		Telepon:     strings.TrimSpace(in.Telepon),
		NPWP:        strings.TrimSpace(in.NPWP),
		KopKwitansi: strings.TrimSpace(in.KopKwitansi),
	}
	if clean.Nama == "" {
		return fmt.Errorf("nama toko wajib diisi")
	}
	raw, err := json.Marshal(&clean)
	if err != nil {
		return fmt.Errorf("marshal toko info: %w", err)
	}
	return s.repo.Set(ctx, domain.SettingKeyTokoInfo, raw, userID)
}

// PajakConfig ambil dari setting; nil-safe (default kalau belum di-set).
func (s *AppSettingService) PajakConfig(ctx context.Context) (*domain.PajakConfig, error) {
	return s.repo.GetPajakConfig(ctx)
}

// UpdatePajakConfig simpan konfigurasi pajak ke key=pajak_config.
func (s *AppSettingService) UpdatePajakConfig(
	ctx context.Context, cfg *domain.PajakConfig, userID *int64,
) error {
	if cfg == nil {
		return fmt.Errorf("pajak config nil")
	}
	clean := domain.PajakConfig{
		PPNEnabled: cfg.PPNEnabled,
		PPNPersen:  cfg.PPNPersen,
		PKP:        cfg.PKP,
		NamaPKP:    strings.TrimSpace(cfg.NamaPKP),
		AlamatPKP:  strings.TrimSpace(cfg.AlamatPKP),
		NPWPPKP:    strings.TrimSpace(cfg.NPWPPKP),
	}
	if clean.PPNPersen < 0 || clean.PPNPersen > 100 {
		return fmt.Errorf("ppn_persen harus 0-100")
	}
	raw, err := json.Marshal(&clean)
	if err != nil {
		return fmt.Errorf("marshal pajak_config: %w", err)
	}
	return s.repo.Set(ctx, domain.SettingKeyPajakConfig, raw, userID)
}

// MarkOnboardingDone set key=onboarding_done -> true.
func (s *AppSettingService) MarkOnboardingDone(ctx context.Context, userID *int64) error {
	raw := json.RawMessage(`true`)
	return s.repo.Set(ctx, domain.SettingKeyOnboardingDone, raw, userID)
}

// IsOnboardingDone passthrough.
func (s *AppSettingService) IsOnboardingDone(ctx context.Context) (bool, error) {
	return s.repo.IsOnboardingDone(ctx)
}

// SMTPConfig load konfigurasi SMTP. Password di-decrypt di sini; kalau
// stored value ternyata legacy plaintext (decrypt gagal), kembalikan apa
// adanya — caller akan re-encrypt pada save berikutnya.
func (s *AppSettingService) SMTPConfig(ctx context.Context) (*domain.SMTPConfig, error) {
	cfg, err := s.repo.GetSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg != nil && cfg.Password != "" {
		plain, _ := crypto.DecryptSecretCompat(cfg.Password)
		cfg.Password = plain
	}
	return cfg, nil
}

// UpdateSMTPConfig simpan konfigurasi SMTP ke key=smtp_config.
func (s *AppSettingService) UpdateSMTPConfig(
	ctx context.Context, in *domain.SMTPConfig, userID *int64,
) error {
	if in == nil {
		return fmt.Errorf("smtp config nil")
	}
	encPwd, err := crypto.EncryptSecret(in.Password) // password jangan di-trim
	if err != nil {
		return fmt.Errorf("encrypt smtp password: %w", err)
	}
	clean := domain.SMTPConfig{
		Host:     strings.TrimSpace(in.Host),
		Port:     strings.TrimSpace(in.Port),
		Username: strings.TrimSpace(in.Username),
		Password: encPwd,
		From:     strings.TrimSpace(in.From),
		Enabled:  in.Enabled,
	}
	raw, err := json.Marshal(&clean)
	if err != nil {
		return fmt.Errorf("marshal smtp config: %w", err)
	}
	return s.repo.Set(ctx, domain.SettingKeySMTPConfig, raw, userID)
}

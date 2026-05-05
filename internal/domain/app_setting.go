package domain

import (
	"encoding/json"
	"time"
)

// AppSetting - row di tabel app_setting (key-value JSONB).
type AppSetting struct {
	ID        int64
	Key       string
	Value     json.RawMessage
	UpdatedAt time.Time
	UpdatedBy *int64
}

// TokoInfo - struktur untuk key "toko_info".
type TokoInfo struct {
	Nama        string `json:"nama"`
	Alamat      string `json:"alamat"`
	Telepon     string `json:"telepon"`
	NPWP        string `json:"npwp"`
	KopKwitansi string `json:"kop_kwitansi"`
}

// PajakConfig - konfigurasi pajak (PPN) toko.
// PPNEnabled = default toggle PPN saat input transaksi.
// PPNPersen  = persentase PPN aktif (default 11.0 untuk 2024+).
// PKP        = status Pengusaha Kena Pajak (mempengaruhi cetak Faktur Pajak).
type PajakConfig struct {
	PPNEnabled bool    `json:"ppn_enabled"`
	PPNPersen  float64 `json:"ppn_persen"`
	PKP        bool    `json:"pkp"`
	NamaPKP    string  `json:"nama_pkp"`
	AlamatPKP  string  `json:"alamat_pkp"`
	NPWPPKP    string  `json:"npwp_pkp"`
}

// SMTPConfig - struktur untuk key "smtp_config".
// Untuk Fase 1, password disimpan plain JSON. Encryption follow-up.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	Enabled  bool   `json:"enabled"`
}

// Setting key constants.
const (
	SettingKeyTokoInfo       = "toko_info"
	SettingKeyOnboardingDone = "onboarding_done"
	SettingKeyPajakConfig    = "pajak_config"
	SettingKeySMTPConfig     = "smtp_config"
)

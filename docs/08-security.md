# 08 — Security

## Authentication

### Password Hashing

- Algoritma: **Argon2id** (bukan bcrypt)
- Parameter: `m=64MB, t=3, p=2` (sesuai OWASP 2025+ rekomendasi)
- Library: `golang.org/x/crypto/argon2`
- Salt: random 16 byte per password
- Hash format: `$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>` (versioned untuk future migration)

### Session Management

- Session disimpan di tabel `session` (DB-backed, bukan JWT)
- Session ID: UUID v4 random
- Cookie: `HttpOnly`, `Secure` (HTTPS only), `SameSite=Lax`
- Expiry: 8 jam, sliding window (refresh setiap request)
- Logout: hapus row session DB (immediate invalidation)
- Cleanup expired session: cron job 1x sehari

```go
// Session cookie set
c.SetCookie(&http.Cookie{
    Name:     "session_id",
    Value:    session.ID,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
    Path:     "/",
    MaxAge:   8 * 3600,
})
```

### Login Throttling

- Max 5 failed attempts per user → lock 30 menit (`user.locked_until`)
- Max 10 failed attempts per IP per 15 menit → IP block sementara
- Reset counter setelah login sukses

### Password Policy

- Minimum 10 karakter
- Wajib: huruf besar + kecil + angka
- Cek breached password via HIBP API (k-anonymity, optional, Fase 6)
- Force change: setelah owner reset password

## Authorization (RBAC)

### Role

| Role | Akses |
|------|-------|
| **Owner** | Semua modul, semua gudang, setting sistem, user management |
| **Admin** | CRUD master data + lihat semua gudang, tidak bisa hapus user |
| **Kasir** | Input penjualan + pembayaran + lihat stok di gudang sendiri |
| **Gudang** | Mutasi + stok opname + lihat stok di gudang sendiri |

### Implementation

Middleware Echo cek role dan scope gudang:

```go
func RequireRole(roles ...string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            user := getCurrentUser(c)
            if !slices.Contains(roles, user.Role) {
                return echo.NewHTTPError(http.StatusForbidden, "akses ditolak")
            }
            return next(c)
        }
    }
}

// Penggunaan
g := e.Group("/admin")
g.Use(RequireRole("owner", "admin"))
```

Scope gudang dicek di service layer:

```go
func (s *PenjualanService) Create(ctx context.Context, input dto.PenjualanCreateInput) error {
    user := authctx.User(ctx)
    if user.Role == "kasir" && user.GudangID != input.GudangID {
        return domain.ErrAksesGudangDitolak
    }
    // ...
}
```

## CSRF Protection

- Middleware Echo `csrf` dengan double-submit cookie
- Token di-inject ke template, dikirim balik via header `X-CSRF-Token` atau form field
- HTMX otomatis kirim token via header config

```go
e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
    TokenLookup:    "header:X-CSRF-Token,form:_csrf",
    CookieName:     "_csrf",
    CookieSecure:   true,
    CookieHTTPOnly: false, // dibutuhkan JavaScript HTMX
    CookieSameSite: http.SameSiteLaxMode,
}))
```

## Audit Log

### Yang Di-log

Setiap operasi mutasi (CREATE, UPDATE, DELETE) di tabel:

- Master: `produk`, `mitra`, `supplier`, `gudang`, `user`, `harga_produk`
- Transaksi: `penjualan`, `pembayaran`, `mutasi_gudang`, `pembelian`, `stok_opname`
- Operasi sensitif: login, logout, password change, role change

Field yang di-record:
- `user_id`, `aksi`, `tabel`, `record_id`
- `payload_before` (state sebelum, JSONB)
- `payload_after` (state sesudah, JSONB)
- `ip`, `user_agent`, `created_at`

### Implementation

Helper di service layer (bukan trigger DB) — agar context user terbawa:

```go
func (s *PenjualanService) Create(ctx context.Context, input dto.PenjualanCreateInput) (*domain.Penjualan, error) {
    return s.tx.RunInTx(ctx, func(ctx context.Context) (*domain.Penjualan, error) {
        // ... logic create ...

        if err := s.audit.Record(ctx, audit.Entry{
            Aksi:          "CREATE",
            Tabel:         "penjualan",
            RecordID:      penjualan.ID,
            PayloadAfter:  penjualan,
        }); err != nil {
            return nil, err
        }

        return penjualan, nil
    })
}
```

### Retensi

- Disimpan **7 tahun** (kewajiban audit pajak Indonesia per UU KUP)
- Archive ke cold storage (parquet) setelah 2 tahun untuk reduce DB size

## Data Protection

### SQL Injection Prevention

- **Hanya pakai prepared statement** via sqlc (auto-escape)
- Tidak ada string concatenation untuk query
- Code review wajib reject PR dengan `fmt.Sprintf` untuk SQL

### XSS Prevention

- Templ default escape semua output (kecuali `templ.Raw` yang **dilarang** untuk user input)
- Content Security Policy strict (lihat Headers di bawah)
- Sanitize user input sebelum simpan (strip HTML tag jika tidak perlu)

### Mass Assignment

- DTO eksplisit untuk input, tidak langsung bind struct domain ke request
- Field yang user tidak boleh set (id, created_at, user_id) dipisah dari DTO

## HTTP Security Headers

Configure di middleware Echo:

```go
e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
    XSSProtection:         "1; mode=block",
    ContentTypeNosniff:    "nosniff",
    XFrameOptions:         "DENY",
    HSTSMaxAge:            31536000,    // 1 year
    HSTSPreloadEnabled:    true,
    ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'",
    ReferrerPolicy:        "strict-origin-when-cross-origin",
}))
```

CSP `'unsafe-inline'` script terpaksa untuk Alpine.js inline; future improvement: nonce-based CSP.

## TLS

- Production: Caddy auto Let's Encrypt (Fase 9)
- Local dev: HTTP saja (cukup untuk testing)
- Versi minimal: TLS 1.2 (TLS 1.3 preferred)
- Cipher suite: modern (Caddy default)

## Database Security

### User & Role

- Aplikasi pakai user `tokobangunan_app` dengan hak terbatas (CRUD ke tabel app, tidak ada DDL)
- Backup pakai user terpisah `tokobangunan_backup` (read-only ke pg_dump)
- Reporting pakai read-only replica (Fase 9+) dengan user `tokobangunan_report`

### Connection

- TLS connection antara app dan DB (`sslmode=require` di production)
- Connection pool: max 25 (sesuai VPS spec), idle timeout 5 menit
- Statement timeout: 30 detik (kill query lambat)

### Backup

- **Strategi 3-2-1**: 3 copy, 2 media beda, 1 off-site
  - Copy 1: pg_dump harian di VPS (7 hari rolling)
  - Copy 2: replikasi ke object storage (S3-compatible, e.g. Wasabi/Backblaze) — encrypted at rest, 90 hari retention
  - Copy 3: backup manual bulanan ke laptop owner (USB external)
- **Schedule**: cron 02:00 WITA setiap hari
- **Test restore**: drill bulanan ke staging (verifikasi backup actually restorable)
- **Encryption**: backup file di-encrypt dengan GPG sebelum upload (`pg_dump | gzip | gpg -c -o ...`)

## Secrets Management

- Environment variable via `.env` file (mode 600, owner only)
- Tidak commit `.env` ke git (`.gitignore`)
- Production: secret di systemd `EnvironmentFile=/etc/tokobangunan.env` (root only)
- Rotate secret: setiap 90 hari (SESSION_SECRET, COOKIE_SECRET, DB_PASSWORD)

## Rate Limiting

- Login endpoint: 10 req per 15 menit per IP
- API umum: 100 req per menit per session
- Public endpoint (jika ada): 30 req per menit per IP
- Library: `github.com/labstack/echo/v4/middleware` rate limit + custom store di Redis (Fase 6+)

## Input Validation & File Upload

### Form Input

- Server-side validation via `validator/v10`
- Client-side validation via Alpine.js (UX, bukan security)
- Length limit: text field max 500 char, textarea max 5000

### File Upload (Fase 5+ jika dibutuhkan)

Untuk upload bukti pembayaran, foto produk:

- MIME type whitelist: `image/jpeg`, `image/png`, `image/webp`, `application/pdf`
- Magic byte validation (jangan trust extension)
- Max size: 5 MB
- Strip EXIF (privacy)
- Simpan dengan nama random UUID (bukan nama original)
- Storage: object storage (S3-compatible), bukan disk lokal
- Antivirus scan via ClamAV (production)

## Monitoring & Alerting

### Yang Di-monitor (Fase 9)

- Failed login spike
- 500 error rate
- DB query slow (> 1 detik)
- Disk usage > 80%
- Memory usage > 80%
- Backup failure

### Tools

- Prometheus + Grafana (self-hosted di VPS)
- Alertmanager → email/Telegram bot
- Uptime monitoring: UptimeRobot atau self-hosted

## Incident Response

### Severity Level

| Level | Definisi | Response Time |
|-------|----------|---------------|
| P0 | Production down, semua user impact | < 30 menit |
| P1 | Modul kritis broken (penjualan, pembayaran) | < 2 jam |
| P2 | Modul non-kritis broken | < 1 hari |
| P3 | Bug minor, workaround ada | < 1 minggu |

### Runbook

Akan ditambah di Fase 9:
- Server down → restart prosedur
- DB corruption → restore dari backup
- Suspected breach → isolate, audit log review, password reset

## Compliance

### UU PDP (Pelindungan Data Pribadi) Indonesia

- Data mitra (nama, alamat, kontak, NPWP) adalah PII
- Akses terbatas role-based
- Audit log retain 7 tahun
- Hak akses & koreksi: mitra bisa request via owner (manual flow)
- Breach notification: ke OJK/Kominfo dalam 72 jam jika terjadi

### Pajak (UU KUP)

- Audit log keuangan retain 10 tahun (untuk transaksi finansial)
- Format laporan harus support PPN/PPh report (Fase 6+)

## Checklist Security Pre-Launch

- [ ] Argon2id implementasi sesuai parameter
- [ ] HTTPS enforced (Caddy auto SSL)
- [ ] CSP header strict
- [ ] CSRF middleware aktif di semua mutating endpoint
- [ ] Rate limit login aktif
- [ ] Audit log cover semua mutasi
- [ ] Backup harian + restore test
- [ ] Secret rotated, tidak ada di git history
- [ ] Penetration test (basic OWASP Top 10) by pihak ke-3 atau internal
- [ ] User training: phishing awareness, password manager

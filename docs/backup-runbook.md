# Backup & Restore Runbook

Operator guide untuk backup database tokobangunan, off-host replication ke
Hostinger, dan restore drill.

## TL;DR

| Action | Command |
| --- | --- |
| Manual backup | `bash scripts/backup.sh` |
| Manual backup + upload | `set -a; source .env; set +a; bash scripts/backup.sh` |
| Install cron (daily 02:00) | `bash scripts/install-cron.sh` |
| Remove cron | `bash scripts/install-cron.sh --remove` |
| Restore latest | `bash scripts/restore.sh` |

## 1. Konfigurasi env

Lihat `.env.example` blok `# ---------- Backup ----------`. Variabel relevan:

```
BACKUP_PASSPHRASE=...                 # opsional, kalau di-set output di-encrypt
BACKUP_SSH_HOST=185.214.124.85
BACKUP_SSH_PORT=65002
BACKUP_SSH_USER=u212852160
BACKUP_SSH_KEY=~/.ssh/hostinger_scriptsis
BACKUP_REMOTE_DIR=~/domains/scriptsis.id/public_html/backups/tokobangunan/
BACKUP_RETAIN_DAYS=30
BACKUP_RETAIN_COUNT=30
```

Kalau salah satu dari `BACKUP_SSH_HOST/USER/KEY` kosong, langkah SCP di-skip
(backup tetap tersimpan lokal).

## 2. Manual backup

```bash
# Plain dump (no encryption, no upload).
bash scripts/backup.sh

# Encrypted + uploaded — load env dulu.
set -a; source .env; set +a
bash scripts/backup.sh
```

Output:
- `./backups/tokobangunan_YYYYMMDD_HHMMSS.dump`              (plain)
- `./backups/tokobangunan_YYYYMMDD_HHMMSS.dump.gz.gpg`        (encrypted)

Log baris berformat `[YYYY-MM-DD HH:MM:SS] ...`.

## 3. Install cron

```bash
bash scripts/install-cron.sh                 # default: 0 2 * * *
CRON_TIME="0 3 * * *" bash scripts/install-cron.sh
crontab -l                                   # verify
bash scripts/install-cron.sh --remove
```

Cron memakai env dari shell saat `bash scripts/backup.sh` jalan — pastikan
`.env` di-load via wrapper, atau export di crontab line. Contoh wrapper:

```cron
0 2 * * * cd /path/tokobangunan && set -a && . ./.env && set +a && bash scripts/backup.sh >> backups/backup.log 2>&1
```

Default `install-cron.sh` belum auto-load `.env` — edit kalau perlu encrypt +
upload otomatis.

Log default: `./backups/backup.log`.

## 4. Restore

### Via CLI (paling cepat)
```bash
# Restore file paling baru di ./backups/.
bash scripts/restore.sh

# Restore file spesifik.
bash scripts/restore.sh ./backups/tokobangunan_20260504_020000.dump.gz.gpg
```

Kalau file ber-suffix `.gpg`, script minta `BACKUP_PASSPHRASE` (env atau
prompt).

### Via Hostinger (off-host file)
```bash
scp -i ~/.ssh/hostinger_scriptsis -P 65002 \
  u212852160@185.214.124.85:~/domains/scriptsis.id/public_html/backups/tokobangunan/tokobangunan_20260504_020000.dump.gz.gpg \
  ./backups/
bash scripts/restore.sh ./backups/tokobangunan_20260504_020000.dump.gz.gpg
```

### Via UI
Belum ada panel admin untuk restore — operasi destructive dilakukan via CLI.

## 5. Off-host destination

- Server: Hostinger shared (`u212852160@185.214.124.85:65002`).
- Path:   `~/domains/scriptsis.id/public_html/backups/tokobangunan/`.
- Akses:  SSH key `~/.ssh/hostinger_scriptsis` (ed25519).
- Catatan: path di bawah `public_html` — pastikan ada `.htaccess` `Deny from
  all` atau pindahkan ke folder di luar webroot kalau privacy concerns.

## 6. Encryption key handling

- `BACKUP_PASSPHRASE` adalah symmetric AES256 passphrase.
- Simpan di password manager (1Password / Bitwarden), JANGAN commit ke repo.
- Rotasi tiap 90 hari atau setelah personnel change.
- Saat rotasi, decrypt file lama → encrypt ulang dengan passphrase baru ATAU
  catat passphrase historis dengan window restore yang jelas.
- Loss of passphrase = permanent data loss untuk file ter-encrypt.

## 7. Retention

- Lokal: prune `> BACKUP_RETAIN_DAYS` hari (default 30) DAN keep maksimal
  `BACKUP_RETAIN_COUNT` (default 30) file terbaru.
- Remote (Hostinger): tidak otomatis. Jadwalkan housekeeping bulanan via
  cron di Hostinger atau script terpisah:

  ```bash
  ssh -i ~/.ssh/hostinger_scriptsis -p 65002 u212852160@185.214.124.85 \
    "find ~/domains/scriptsis.id/public_html/backups/tokobangunan/ \
       -name 'tokobangunan_*' -type f -mtime +60 -delete"
  ```

## 8. Restore drill (CI)

Workflow `.github/workflows/restore-test.yml` jalan tiap Minggu 04:00 UTC
(dan manual via "Run workflow"). Steps:

1. Spin Postgres 16 service.
2. Apply migrations dari `db/migrations/*.up.sql`.
3. `pg_dump` → file dump.
4. Buat database baru `tokobangunan_drill`, `pg_restore` ke situ.
5. Smoke check `count(*)` tabel.

Failure → buka issue manual; biasanya schema drift atau migration ordering.

## 9. Troubleshooting

| Gejala | Cek |
| --- | --- |
| `docker exec ... not running` | `docker compose up -d db` dulu |
| `Permission denied (publickey)` | path `BACKUP_SSH_KEY` benar? `chmod 600`? |
| `gpg: decryption failed` | passphrase salah / corrupt download |
| Cron tidak jalan | `grep CRON /var/log/syslog`, env tidak ter-load → pakai wrapper |
| File `.dump` 0 byte | container DB down saat dump → cek log container |

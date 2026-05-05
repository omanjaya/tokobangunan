# Disaster Recovery Runbook

Operator guide untuk pemulihan sistem Tokobangunan ERP saat terjadi insiden major.
Dokumen ini di-design untuk dibaca **saat panik** — perintah konkret, copy-pastable.

> Cross-reference: lihat juga [`backup-runbook.md`](./backup-runbook.md) untuk
> mekanisme backup harian dan restore drill.

---

## 1. Scope

Runbook ini meng-cover skenario disaster berikut:

| Kategori | Contoh |
| --- | --- |
| **DB corruption** | PostgreSQL crash, page corruption, FS error, WAL invalid |
| **App server crash** | Container loop, OOM, panic berulang, deadlock |
| **Data loss** | Hapus penjualan/pembayaran tidak sengaja, drop table, bug logic |
| **Security breach** | Kredensial bocor, akses unauthorized, suspected exfiltrasi |
| **Hardware/network failure** | Server mati, datacenter outage, jaringan unreachable |

Out of scope: bug functional minor (pakai workflow normal), feature request,
performance tuning rutin.

---

## 2. RPO / RTO Targets

| Metric | Target | Catatan |
| --- | --- | --- |
| **RPO** (Recovery Point Objective) | **24 jam** | Backup harian 02:00 WIB, max kehilangan data 1 hari |
| **RTO** (Recovery Time Objective) | **1 jam** | Dari detect → restore → redeploy → service hijau |
| **MTTR target** (mean time to recovery) | < 30 menit | Untuk skenario app crash sederhana |

Jika insiden butuh > 1 jam, eskalasi ke owner segera dan komunikasikan ke staff
bahwa sistem offline.

---

## 3. Backup Verification

Sebelum restore, **selalu** verifikasi backup yang akan dipakai valid:

```bash
# List backup tersedia di server lokal
ls -lh backups/

# Verifikasi integrity backup paling baru
bash scripts/verify-backup.sh backups/tokobangunan-$(date +%F).sql.gz.gpg

# Cek backup off-host (Hostinger)
ssh -i ~/.ssh/hostinger_scriptsis -p 65002 u212852160@185.214.124.85 \
  "ls -lh ~/domains/scriptsis.id/public_html/backups/tokobangunan/ | tail -10"
```

Detail prosedur backup & restore drill ada di [`backup-runbook.md`](./backup-runbook.md).
**CI restore-test** jalan otomatis tiap minggu di
[`.github/workflows/restore-test.yml`](../.github/workflows/restore-test.yml).

---

## 4. Recovery Procedures

### 4.1 DB Corruption

**Gejala:** PostgreSQL log error `invalid page header`, `could not read block`,
queries fail dengan `database disk image is malformed`, app `/readyz` merah.

```bash
# 1. Stop app supaya tidak ada write baru ke DB rusak
docker compose stop app

# 2. Snapshot state DB rusak (untuk forensik nanti)
docker compose exec db pg_dump -U postgres tokobangunan > /tmp/corrupt-state-$(date +%s).sql || true

# 3. Stop & hapus volume DB
docker compose stop db
docker volume rm tokobangunan_db-data || docker volume rm tokobangunan_pgdata

# 4. Start fresh DB
docker compose up -d db
sleep 10  # tunggu DB ready

# 5. Restore backup terbaru
bash scripts/restore.sh

# 6. Verifikasi data integrity
docker compose exec db psql -U postgres tokobangunan -c "SELECT COUNT(*) FROM penjualan;"
docker compose exec db psql -U postgres tokobangunan -c "SELECT MAX(tanggal) FROM penjualan;"

# 7. Restart app
docker compose up -d app

# 8. Smoke test
curl -fsS http://localhost:7777/readyz
curl -fsS http://localhost:7777/livez
```

**Eskalasi:** Jika restore gagal (backup juga corrupt), pakai backup H-1, H-2.
Jika semua gagal → eskalasi ke owner, siapkan komunikasi data loss ke staff.

---

### 4.2 App Crash

**Gejala:** `/livez` atau `/readyz` 5xx, container restart loop, response timeout.

```bash
# 1. Cek status container
docker compose ps

# 2. Cek log error terakhir
docker compose logs --tail=200 app

# 3. Restart app (cara cepat)
docker compose restart app

# 4. Tunggu ready
for i in {1..30}; do
  curl -fsS http://localhost:7777/readyz && break
  sleep 2
done
```

**Jika persistent (restart ke-3 gagal dalam 5 menit):**

```bash
# Full rebuild
docker compose down app
docker compose up -d --force-recreate app
docker compose logs -f app
```

**Eskalasi:** Jika setelah rebuild masih crash, kemungkinan masalah di DB
(lihat 4.1) atau bug release terbaru → rollback image:

```bash
# Rollback ke image sebelumnya
docker compose down app
docker tag tokobangunan-app:previous tokobangunan-app:latest
docker compose up -d app
```

---

### 4.3 Data Loss (Accidental Delete)

**Gejala:** User report data hilang (penjualan, pembayaran, produk), bug
hapus massal, drop tidak disengaja.

#### Sub-1 jam (data hilang < 1 jam yang lalu)

Prioritas: **jangan restore full backup** karena akan lose data 24 jam.

**Opsi A — App-level cancel/restore:**
```
- Penjualan → pakai fitur "Batalkan" (POST /penjualan/:id/cancel)
- Pembayaran → tandai void via UI admin
- Produk soft-deleted → restore via setting/produk
```

**Opsi B — DB transaction logs (jika PITR enabled):**
```bash
# Cek apakah PITR enabled
docker compose exec db psql -U postgres -c "SHOW archive_mode;"

# Jika 'on', recover ke titik sebelum delete
# Stop app, replay WAL ke timestamp target
docker compose stop app
docker compose exec db psql -U postgres -c "SELECT pg_wal_replay_pause();"
# Lihat dokumentasi PostgreSQL PITR untuk replay sampai timestamp
```

**Opsi C — Restore DB ke staging, copy data manual:**
```bash
# 1. Restore backup ke instance staging
docker run -d --name pg-staging -e POSTGRES_PASSWORD=x -p 5433:5432 postgres:16
gunzip -c backups/tokobangunan-latest.sql.gz | docker exec -i pg-staging psql -U postgres

# 2. Copy row yang hilang ke production
docker exec pg-staging pg_dump -U postgres -t penjualan --where="id IN (...)" > /tmp/recover.sql
docker compose exec -T db psql -U postgres tokobangunan < /tmp/recover.sql
```

#### >1 jam (data hilang lebih dari 1 jam)

Trade-off: restore backup → kehilangan transaksi setelah backup snapshot.

```bash
# 1. Komunikasikan ke staff: "Sistem akan offline X menit, hentikan input baru"

# 2. Export delta dari production (transaksi setelah backup snapshot)
docker compose exec db pg_dump -U postgres tokobangunan \
  --table=penjualan --table=pembayaran \
  --where="created_at > '$(date -d 'today 02:00' --iso-8601=seconds)'" \
  > /tmp/delta-$(date +%s).sql

# 3. Restore backup
docker compose stop app
bash scripts/restore.sh

# 4. Replay delta MANUAL (review dulu — jangan blind apply)
less /tmp/delta-*.sql
# edit untuk skip row yang hilang, apply sisanya
docker compose exec -T db psql -U postgres tokobangunan < /tmp/delta-edited.sql

# 5. Restart & verifikasi
docker compose up -d app
```

---

### 4.4 Security Breach

**Gejala:** Login dari IP asing di audit log, password leak suspected,
akses tidak wajar di Hostinger panel, malware detection.

**Langkah cepat (dalam 15 menit):**

```bash
# 1. ROTATE SESSION SECRET — logout semua user aktif
openssl rand -hex 32  # generate baru
# Update SESSION_SECRET di .env, restart app
docker compose restart app

# 2. ROTATE BACKUP PASSPHRASE
openssl rand -base64 48
# Update BACKUP_PASSPHRASE di .env (catat lama untuk decrypt backup historis)

# 3. ROTATE DB PASSWORD
docker compose exec db psql -U postgres -c "ALTER USER tokobangunan WITH PASSWORD 'NEW_STRONG_PASSWORD';"
# Update DB_PASSWORD di .env, restart app
docker compose restart app

# 4. ROTATE HOSTINGER SSH KEY
ssh-keygen -t ed25519 -f ~/.ssh/hostinger_scriptsis_new -N ""
# Tambahkan pubkey baru via Hostinger panel → SSH Keys
# Hapus key lama dari panel
mv ~/.ssh/hostinger_scriptsis ~/.ssh/hostinger_scriptsis.compromised
mv ~/.ssh/hostinger_scriptsis_new ~/.ssh/hostinger_scriptsis

# 5. AUDIT LOG REVIEW — cek aktivitas user 7 hari terakhir
docker compose exec db psql -U postgres tokobangunan -c "
  SELECT user_id, action, target, ip, ts
  FROM audit_log
  WHERE ts > now() - interval '7 days'
  ORDER BY ts DESC LIMIT 200;"

# 6. Cek session aktif
docker compose exec db psql -U postgres tokobangunan -c "
  SELECT user_id, ip, last_seen FROM session_active ORDER BY last_seen DESC;"
```

**Pasca-rotasi:**
- Inform semua user: password mereka di-reset, harus login ulang
- Force password reset semua role `owner` dan `admin`
- Review file `.env` untuk kredensial lain yang perlu rotasi (SMTP, dll)
- Buat issue forensik: catat timeline, IP source, scope dampak

---

### 4.5 Hardware / Network Failure

**Gejala:** Server unreachable > 10 menit, datacenter outage, hardware fail.

```bash
# 1. Provision server pengganti (misalnya VPS Hostinger baru atau cloud lain)
# - install Docker + docker compose
# - clone repo: git clone <repo-url>
# - copy .env (dari secure store, jangan commit)

# 2. Pull backup paling baru dari off-host (Hostinger)
ssh -i ~/.ssh/hostinger_scriptsis -p 65002 u212852160@185.214.124.85 \
  "ls -t ~/domains/scriptsis.id/public_html/backups/tokobangunan/*.sql.gz.gpg | head -1" \
  | xargs -I {} scp -i ~/.ssh/hostinger_scriptsis -P 65002 \
    u212852160@185.214.124.85:{} ./backups/

# 3. Boot stack di server baru
docker compose up -d db
sleep 10
bash scripts/restore.sh
docker compose up -d

# 4. Verifikasi
curl -fsS http://localhost:7777/readyz

# 5. Repoint DNS — update A record domain ke IP server baru
# Hostinger panel → DNS Zone Editor → edit A record
# atau via Hostinger MCP tool

# 6. Tunggu DNS propagasi (TTL biasanya 300-3600s)
dig +short app.tokobangunan.com
```

---

## 5. Communication Plan

| Severity | Notifikasi ke | Channel | Waktu |
| --- | --- | --- | --- |
| **P0** (data loss, breach) | Owner | Telepon + WA | < 5 menit |
| **P1** (system down >15min) | Owner + Admin | WA grup | < 15 menit |
| **P2** (degraded, partial outage) | Admin | WA grup | < 30 menit |
| **P3** (resolved insiden) | Semua staff | WA grup broadcast | Setelah recovery |

**Template pesan staff (P1/P2):**
```
[TOKOBANGUNAN] Sistem ERP sedang dipulihkan. Estimasi selesai HH:MM.
Sementara waktu, catat transaksi manual di buku, input setelah sistem hidup.
```

**Template pesca-resolved:**
```
[TOKOBANGUNAN] Sistem sudah pulih. Mohon input transaksi manual yang sempat
dicatat. Jika ada data hilang, lapor ke admin.
```

---

## 6. Test Schedule

| Tipe drill | Frekuensi | Owner | Trigger |
| --- | --- | --- | --- |
| Restore test (otomatis) | Mingguan | CI | [`.github/workflows/restore-test.yml`](../.github/workflows/restore-test.yml) |
| Full DR drill (manual) | Quarterly | Owner | Q1, Q2, Q3, Q4 |
| Security rotation drill | Semi-annual | Owner | Juni, Desember |

**Quarterly DR drill checklist:**
- [ ] Stop production replica (atau pakai staging)
- [ ] Provision server bersih
- [ ] Restore backup paling baru
- [ ] Verifikasi data integrity (`SELECT COUNT(*)` semua tabel utama)
- [ ] Smoke test semua endpoint kritis (`/login`, `/penjualan`, `/pembayaran`)
- [ ] Catat waktu total RTO aktual
- [ ] Bandingkan dengan target RTO 1 jam
- [ ] Update runbook jika ada gap

---

## 7. Post-Incident

Setelah insiden teratasi, **wajib** dalam 48 jam:

### 7.1 Retrospective

Buat dokumen `docs/incidents/YYYY-MM-DD-slug.md` berisi:
- **Timeline** — detik per detik: detect, diagnose, mitigate, resolve
- **Root cause** — apa yang sebenarnya terjadi (5 whys)
- **Impact** — berapa user terdampak, data hilang, downtime
- **What went well** — proses yang membantu
- **What went wrong** — hambatan, miss
- **Action items** — perbaikan dengan owner & due date

### 7.2 Update Runbook

- Jika ada langkah yang tidak akurat → update bagian terkait
- Jika ada skenario baru tidak ter-cover → tambah section
- Jika ada command yang lebih efisien → ganti

### 7.3 Tech Debt

- Buat issue untuk preventive measures (monitoring gap, missing alert)
- Schedule fix sesuai prioritas
- Review apakah RPO/RTO target masih realistis

---

## Appendix: Quick Reference Card

```
┌──────────────────────────────────────────────────────────────┐
│ EMERGENCY HOTLINE: <owner-phone>                             │
│ HOSTINGER PANEL:   https://hpanel.hostinger.com              │
│ SSH:               ssh -i ~/.ssh/hostinger_scriptsis -p 65002 │
│                       u212852160@185.214.124.85              │
│ HEALTH:            curl http://localhost:7777/readyz          │
│ RESTORE:           bash scripts/restore.sh                    │
│ BACKUP NOW:        bash scripts/backup.sh                     │
│ LOGS:              docker compose logs -f app                 │
└──────────────────────────────────────────────────────────────┘
```

# Production Deployment Runbook

Operator guide untuk deploy tokobangunan ke server produksi.

## 1. Prerequisites

- Docker + Docker Compose v2
- PostgreSQL 16 (managed atau container) reachable dari host
- Reverse proxy (Caddy / nginx) untuk TLS termination — server Echo tidak
  handle TLS sendiri
- Host minimal 2 GB RAM, 1 vCPU, 20 GB disk
- Outbound SSH access ke Hostinger untuk off-host backup

## 2. First-time setup

1. Clone repo:
   ```bash
   git clone <repo-url> /opt/tokobangunan
   cd /opt/tokobangunan
   ```

2. Generate `.env`:
   ```bash
   cp .env.production.example .env
   chmod 600 .env
   # isi DATABASE_URL, SESSION_SECRET (openssl rand -base64 32), dst.
   ```

3. Generate `.env.backup`:
   ```bash
   bash scripts/gen-backup-secrets.sh > .env.backup
   chmod 600 .env.backup
   # simpan BACKUP_PASSPHRASE ke password manager — loss = data loss permanen
   ```

4. Generate `/metrics` credentials (kalau di-scrape eksternal):
   ```bash
   bash scripts/gen-metrics-secrets.sh >> .env
   ```

5. Build production image:
   ```bash
   docker build -f Dockerfile -t tokobangunan:prod .
   ```

6. Run via compose dengan override prod (skip air, jalankan binary):
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
   ```

7. Apply migrations:
   ```bash
   docker exec -i tokobangunan-app \
     migrate -path db/migrations -database "$DATABASE_URL" up
   ```

8. Seed data awal (kalau ada command seed):
   ```bash
   docker exec -it tokobangunan-app /server seed
   ```

9. Selesaikan onboarding flow di `https://<domain>/onboarding` (set
   business profile, admin user, opening balance).

## 3. Reverse proxy

Contoh Caddy (`/etc/caddy/Caddyfile`):

```
tokobangunan.com {
  reverse_proxy localhost:8080
  encode gzip zstd
}
```

Caddy auto-handle TLS (Let's Encrypt) + HSTS. Echo middleware juga inject
HSTS header sebagai defense-in-depth.

Contoh nginx snippet:
```nginx
server {
  listen 443 ssl http2;
  server_name tokobangunan.com;
  ssl_certificate     /etc/letsencrypt/live/tokobangunan.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/tokobangunan.com/privkey.pem;
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

## 4. Cron setup (backup harian 02:00)

```bash
bash scripts/install-cron.sh
crontab -l   # verify
```

Detail: lihat [backup-runbook.md](./backup-runbook.md).

## 5. Monitoring

- Endpoint `/metrics` ekspos format Prometheus.
- Scrape config:
  ```yaml
  scrape_configs:
    - job_name: tokobangunan
      basic_auth:
        username: ${METRICS_USER}
        password: ${METRICS_PASS}
      static_configs:
        - targets: ['tokobangunan.com:443']
      scheme: https
  ```
- Dashboard Grafana sederhana: latency p50/p95, request rate, error rate
  (5xx), DB pool saturation, memory RSS, goroutine count.
- Alerting: 5xx > 1%/5min, readyz down 2 menit, disk usage > 80%.

## 6. Rotation runbook

| Secret | Frekuensi | Efek samping |
| --- | --- | --- |
| `SESSION_SECRET` | 90 hari atau setelah leak | Logout semua user |
| `BACKUP_PASSPHRASE` | 90 hari | Backup lama tetap pakai key lama — dokumentasikan window |
| DB password | 90 hari | Update `DATABASE_URL`, restart app |
| SSH key Hostinger | 180 hari | Re-distribute `authorized_keys`, update `BACKUP_SSH_KEY` |
| `METRICS_USER/PASS` | 180 hari | Update Prometheus scrape config |

Prosedur rotate `SESSION_SECRET`:
```bash
NEW=$(openssl rand -base64 32)
sed -i "s|^SESSION_SECRET=.*|SESSION_SECRET=$NEW|" .env
docker compose restart app
```

Prosedur rotate `BACKUP_PASSPHRASE`:
1. Generate baru: `bash scripts/gen-backup-secrets.sh`
2. Update `.env.backup`.
3. Catat passphrase lama + tanggal cutoff di password manager (untuk
   restore arsip lama).
4. Backup berikutnya akan pakai key baru.

## 7. Troubleshooting

| Gejala | Diagnostik | Fix |
| --- | --- | --- |
| Container restart loop | `docker logs tokobangunan-app` | Lihat panic / config error |
| `/readyz` 503 | `docker exec ... pg_isready` | DB unreachable, cek `DATABASE_URL` & network |
| 429 dari endpoint | User complain rate-limit | Naikkan limit di `cmd/server/main.go` middleware atau scale up |
| TLS error di reverse proxy | `caddy validate` / `nginx -t` | Cek cert + DNS A record |
| Backup tidak upload | `tail backups/backup.log` | SSH key path absolute? `chmod 600` key? |
| OOM kill | `dmesg | grep -i kill` | Naikkan memory limit, profile heap via `/debug/pprof` |

## 8. Related docs

- [backup-runbook.md](./backup-runbook.md) — backup operations
- [disaster-recovery.md](./disaster-recovery.md) — DR procedure
- [partition-runbook.md](./partition-runbook.md) — DB partition rollover
- [openapi.yaml](./openapi.yaml) — API contract

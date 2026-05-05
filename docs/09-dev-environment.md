# 09 — Dev Environment

Setup development lokal dengan Docker Compose + Air (hot reload). **Fokus: lokal dulu, deployment VPS ditunda sampai MVP siap.**

## Prerequisites

| Tool | Versi Min | Catatan |
|------|-----------|---------|
| Docker | 24+ | Docker Desktop (Mac/Windows) atau Docker Engine + Compose v2 (Linux) |
| Git | 2.30+ | Clone repo |
| Editor | - | VSCode + extension `templ-go.templ` direkomendasikan |
| Browser | Chrome/Firefox latest | Test UI |

Tidak perlu install Go, PostgreSQL, atau Air di host — semua jalan di container.

## Quick Start

```bash
# 1. Clone repo
cd /Users/omanjaya/project/tokobangunan

# 2. Setup environment
cp .env.example .env

# 3. Start semua container
make up

# 4. Run migrasi DB
make migrate

# 5. Seed data master (gudang, satuan, user owner default)
make seed

# 6. Buka aplikasi
# App:     http://localhost:8080
# Adminer: http://localhost:8081  (DB GUI)
```

Setelah ini, edit file Go atau Templ → Air detect → rebuild < 2 detik → refresh browser.

## File Konfigurasi

### `docker-compose.yml`

```yaml
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile.dev
    container_name: tokobangunan-app
    ports:
      - "8080:8080"
    volumes:
      - .:/app
      - go-mod-cache:/go/pkg/mod
      - go-build-cache:/root/.cache/go-build
    environment:
      - DATABASE_URL=postgres://dev:dev@db:5432/tokobangunan?sslmode=disable
      - APP_ENV=development
      - LOG_LEVEL=debug
      - LOG_FORMAT=text
      - SESSION_SECRET=dev-only-secret-do-not-use-in-prod
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    container_name: tokobangunan-db
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_USER=dev
      - POSTGRES_PASSWORD=dev
      - POSTGRES_DB=tokobangunan
    volumes:
      - pg-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dev -d tokobangunan"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  adminer:
    image: adminer:latest
    container_name: tokobangunan-adminer
    ports:
      - "8081:8080"
    environment:
      - ADMINER_DEFAULT_SERVER=db
      - ADMINER_DESIGN=hever
    depends_on:
      - db
    restart: unless-stopped

volumes:
  go-mod-cache:
  go-build-cache:
  pg-data:
```

### `Dockerfile.dev`

```dockerfile
FROM golang:1.22-alpine

RUN apk add --no-cache git make curl

# Install Air untuk hot reload
RUN go install github.com/air-verse/air@latest

# Install Templ untuk type-safe template
RUN go install github.com/a-h/templ/cmd/templ@latest

# Install golang-migrate
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Install sqlc
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

WORKDIR /app

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]
```

### `.air.toml`

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "templ generate && go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "docs"]
  exclude_file = []
  exclude_regex = ["_test.go", ".*_templ.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html", "templ", "sql"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = true

[screen]
  clear_on_rebuild = true
```

### `.env.example`

```env
# Database
DATABASE_URL=postgres://dev:dev@db:5432/tokobangunan?sslmode=disable

# Application
APP_ENV=development
APP_PORT=8080
LOG_LEVEL=debug
LOG_FORMAT=text

# Security (generate via: openssl rand -hex 32)
SESSION_SECRET=replace-with-random-32-byte-hex

# Optional
SMTP_HOST=
SMTP_PORT=
SMTP_USER=
SMTP_PASS=
```

### `Makefile`

```makefile
.PHONY: help up down restart logs shell migrate migrate-down seed test lint sqlc-gen templ-gen build clean

help:
	@echo "Available commands:"
	@echo "  make up           - Start all containers"
	@echo "  make down         - Stop all containers"
	@echo "  make restart      - Restart app container"
	@echo "  make logs         - Tail app logs"
	@echo "  make shell        - Shell into app container"
	@echo "  make migrate      - Run DB migrations up"
	@echo "  make migrate-down - Rollback last migration"
	@echo "  make seed         - Seed master data"
	@echo "  make test         - Run all tests"
	@echo "  make lint         - Run linter"
	@echo "  make sqlc-gen     - Generate Go code from SQL queries"
	@echo "  make templ-gen    - Generate Go code from .templ files"
	@echo "  make build        - Build production binary"
	@echo "  make clean        - Remove containers and volumes"

up:
	docker compose up -d
	@echo "App: http://localhost:8080"
	@echo "Adminer: http://localhost:8081 (server=db, user=dev, pass=dev)"

down:
	docker compose down

restart:
	docker compose restart app

logs:
	docker compose logs -f app

shell:
	docker compose exec app sh

migrate:
	docker compose exec app migrate -path db/migrations -database "$$DATABASE_URL" up

migrate-down:
	docker compose exec app migrate -path db/migrations -database "$$DATABASE_URL" down 1

seed:
	docker compose exec app go run ./cmd/seed

test:
	docker compose exec app go test ./... -short

lint:
	docker compose exec app go vet ./...
	docker compose exec app sh -c "command -v golangci-lint && golangci-lint run || echo 'install golangci-lint'"

sqlc-gen:
	docker compose exec app sqlc generate -f db/sqlc.yaml

templ-gen:
	docker compose exec app templ generate

build:
	go build -o ./bin/server ./cmd/server

clean:
	docker compose down -v
	rm -rf tmp bin
```

### `.gitignore`

```
# Binary
/bin/
/tmp/
*.exe
main

# Go
*.test
*.out
coverage.out
coverage.html
vendor/

# Generated
*_templ.go
internal/db/*.go
!internal/db/db.go

# Env
.env
.env.*
!.env.example

# IDE
.vscode/
.idea/
*.swp

# OS
.DS_Store
Thumbs.db

# Logs
*.log
build-errors.log

# Air
tmp/
```

### `.editorconfig`

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
indent_style = space
indent_size = 4
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab

[*.{md,yml,yaml,json,html,templ}]
indent_size = 2

[Makefile]
indent_style = tab
```

## Workflow Developer

### Hari Pertama

```bash
git clone <repo-url> tokobangunan
cd tokobangunan
cp .env.example .env
make up        # Build image + start container (~3 menit pertama, cached setelahnya)
make migrate   # Buat schema DB
make seed      # Seed master data + user owner
```

Login default: `owner` / password ditampilkan di console saat seed.

### Daily Loop

```bash
make up        # Start (jika belum)
make logs      # Tail log untuk lihat error
# ... edit code ...
# Air auto-rebuild + restart, browser refresh manual
make test      # Sebelum commit
make lint      # Sebelum push
```

### Membuat Migrasi Baru

```bash
make shell
migrate create -ext sql -dir db/migrations -seq nama_perubahan
# Edit file .up.sql dan .down.sql
exit
make migrate
```

### Membuat Query Baru

```bash
# 1. Edit db/queries/<table>.sql, tambah query dengan annotation
#    -- name: GetPenjualanByID :one
#    SELECT * FROM penjualan WHERE id = $1;

# 2. Generate Go code
make sqlc-gen

# 3. Pakai di repo:
#    row, err := r.q.GetPenjualanByID(ctx, id)
```

### Membuat Komponen Templ Baru

```bash
# 1. Buat file internal/view/<komponen>.templ
# 2. Air auto-detect, jalankan templ generate
# 3. Pakai di handler atau template lain
```

## Troubleshooting

### Container app tidak start

```bash
make logs
# Cek error build (templ atau go build)
```

### DB connection refused

```bash
docker compose ps
# Pastikan db status "healthy"
docker compose logs db
```

### Port 8080 / 5432 / 8081 sudah dipakai

Edit `docker-compose.yml`, ganti port mapping di kiri:
```yaml
ports:
  - "9080:8080"  # gunakan port lain
```

### Migrasi error "dirty database"

```bash
make shell
migrate -path db/migrations -database "$DATABASE_URL" force <version_sebelumnya>
exit
make migrate
```

### Air tidak detect perubahan file

Pastikan path di `.air.toml` `include_ext` cover ekstensi file yang diubah. Restart container kalau perlu: `make restart`.

### Performance lambat di Mac

Volume mount native lambat di Docker Desktop Mac. Solusi:
- Aktifkan VirtioFS di Docker Desktop settings
- Atau pakai `delegated` consistency: `volumes: [.:/app:delegated]`

## VSCode Setup (Recommended)

### Extension

- Go (golang.go)
- Templ (templ-go.templ)
- Tailwind CSS IntelliSense (bradlc.vscode-tailwindcss)
- PostgreSQL (cweijan.vscode-postgresql-client2) — connect ke DB GUI dari VSCode
- EditorConfig (editorconfig.editorconfig)

### Settings (`.vscode/settings.json`)

```json
{
  "go.formatTool": "gofmt",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "[templ]": {
    "editor.defaultFormatter": "templ-go.templ"
  },
  "files.associations": {
    "*.templ": "templ"
  }
}
```

## Catatan: Tidak Ada di Fase Dev

Hal-hal berikut **tidak** disetup di fase dev — masuk Fase 9 Deployment:

- HTTPS / TLS
- Caddy reverse proxy
- systemd service file
- pg_dump cron backup
- Log aggregation (Loki / Promtail)
- Metrics (Prometheus)
- Production-grade secrets management
- Rate limiting Redis-backed
- CDN / static asset optimization

Lokal cukup pakai HTTP, single-container DB, log ke stdout. Fase 9 baru tambahkan layer production.

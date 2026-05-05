.PHONY: help up down restart logs shell migrate migrate-down seed test lint sqlc-gen templ-gen tailwind-build tailwind-watch build clean migrate-excel-build migrate-excel-audit migrate-excel-dry-run migrate-excel-import backup restore

EXCEL_SOURCE ?= /Users/omanjaya/Downloads/PROJECT UNTUK TOKOBANGUNAN
EXCEL_BIN ?= ./bin/migrate-excel

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
	@echo "  make tailwind-build - Build minified Tailwind CSS (one-shot)"
	@echo "  make tailwind-watch - Watch Tailwind CSS during development"
	@echo "  make build        - Build production binary"
	@echo "  make clean        - Remove containers and volumes"

up:
	docker compose up -d
	@echo ""
	@echo "App:     http://localhost:8080"
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
	docker compose exec app sh -c "command -v golangci-lint && golangci-lint run || echo 'install golangci-lint inside container'"

sqlc-gen:
	docker compose exec app sqlc generate -f db/sqlc.yaml

templ-gen:
	docker compose exec app templ generate

tailwind-watch:
	docker compose exec app sh -c "tailwindcss -i web/static/css/input.css -o web/static/css/app.css --watch"

tailwind-build:
	docker compose exec app sh -c "tailwindcss -i web/static/css/input.css -o web/static/css/app.css --minify"

build:
	go build -o ./bin/server ./cmd/server

clean:
	docker compose down -v
	rm -rf tmp bin

# --- Excel migration (Fase 7) -------------------------------------------------
# Tool berjalan native di host; baca docs di internal/importer/excel/README.md.

migrate-excel-build:
	go build -o $(EXCEL_BIN) ./cmd/migrate-excel

migrate-excel-audit: migrate-excel-build
	$(EXCEL_BIN) --source "$(EXCEL_SOURCE)" --mode audit

migrate-excel-dry-run: migrate-excel-build
	$(EXCEL_BIN) --source "$(EXCEL_SOURCE)" --mode dry-run --confirm-sayan SAYAN

migrate-excel-import: migrate-excel-build
	$(EXCEL_BIN) --source "$(EXCEL_SOURCE)" --mode import --confirm-sayan SAYAN \
	    --year 2025 --batch-size 1000 \
	    --log-file migrate-$$(date +%s).log

# --- Database backup ---------------------------------------------------------
backup:
	bash scripts/backup.sh

restore:
	@if [ -z "$(FILE)" ]; then echo "Usage: make restore FILE=<backup>"; exit 1; fi
	bash scripts/restore.sh "$(FILE)"

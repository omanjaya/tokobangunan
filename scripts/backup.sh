#!/usr/bin/env bash
set -euo pipefail

DB_USER="${DB_USER:-dev}"
DB_NAME="${DB_NAME:-tokobangunan}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
CONTAINER="${DB_CONTAINER:-tokobangunan-db}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

if [[ -n "${BACKUP_PASSPHRASE:-}" ]]; then
    OUT="$BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump.gz.gpg"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F custom \
        | gzip \
        | gpg --batch --yes --quiet \
              --passphrase-file <(printf '%s' "$BACKUP_PASSPHRASE") \
              --symmetric --cipher-algo AES256 \
              -o "$OUT"
    PATTERN="tokobangunan_*.dump.gz.gpg"
else
    OUT="$BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F custom \
        > "$OUT"
    PATTERN="tokobangunan_*.dump"
fi

# Cleanup: keep 30 backup terakhir (sesuai pattern aktif).
find "$BACKUP_DIR" -name "$PATTERN" -type f \
    | sort -r | tail -n +31 | xargs -r rm -f

echo "Backup: $OUT"

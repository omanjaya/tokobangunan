#!/usr/bin/env bash
set -euo pipefail

DB_USER="${DB_USER:-dev}"
DB_NAME="${DB_NAME:-tokobangunan}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
CONTAINER="${DB_CONTAINER:-tokobangunan-db}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F custom \
    > "$BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump"

# Cleanup: keep 30 backup terakhir.
find "$BACKUP_DIR" -name "tokobangunan_*.dump" -type f \
    | sort -r | tail -n +31 | xargs -r rm -f

echo "Backup: $BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump"

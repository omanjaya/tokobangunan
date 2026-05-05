#!/usr/bin/env bash
set -euo pipefail

if [ -z "${1:-}" ]; then
    echo "Usage: $0 <backup-file.dump>"
    exit 1
fi

BACKUP_FILE="$1"
DB_USER="${DB_USER:-dev}"
DB_NAME="${DB_NAME:-tokobangunan}"
CONTAINER="${DB_CONTAINER:-tokobangunan-db}"

docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --clean --if-exists \
    < "$BACKUP_FILE"

echo "Restored from $BACKUP_FILE"

#!/usr/bin/env bash
set -euo pipefail

if [ -z "${1:-}" ]; then
    echo "Usage: $0 <backup-file.dump|.dump.gz|.dump.gz.gpg>"
    exit 1
fi

BACKUP_FILE="$1"
DB_USER="${DB_USER:-dev}"
DB_NAME="${DB_NAME:-tokobangunan}"
CONTAINER="${DB_CONTAINER:-tokobangunan-db}"

case "$BACKUP_FILE" in
    *.dump.gz.gpg)
        if [[ -z "${BACKUP_PASSPHRASE:-}" ]]; then
            echo "ERROR: BACKUP_PASSPHRASE wajib di-set untuk restore file .gpg" >&2
            exit 1
        fi
        gpg --batch --yes --quiet \
            --passphrase-file <(printf '%s' "$BACKUP_PASSPHRASE") \
            --decrypt "$BACKUP_FILE" \
            | gunzip \
            | docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --clean --if-exists
        ;;
    *.dump.gz)
        gunzip -c "$BACKUP_FILE" \
            | docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --clean --if-exists
        ;;
    *.dump)
        docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --clean --if-exists \
            < "$BACKUP_FILE"
        ;;
    *)
        echo "ERROR: ekstensi backup tidak dikenal: $BACKUP_FILE" >&2
        exit 1
        ;;
esac

echo "Restored from $BACKUP_FILE"

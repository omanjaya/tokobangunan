#!/usr/bin/env bash
set -euo pipefail

# Tokobangunan DB backup with optional GPG encrypt + SCP off-host upload.
#
# Env vars:
#   DB_USER, DB_NAME, DB_CONTAINER     - source DB (defaults: dev/tokobangunan/tokobangunan-db)
#   BACKUP_DIR                         - local backup dir (default: ./backups)
#   BACKUP_RETAIN_DAYS                 - prune local files older than N days (default: 30)
#   BACKUP_RETAIN_COUNT                - keep last N files locally (default: 30)
#   BACKUP_PASSPHRASE                  - if set, GPG-encrypt with AES256
#   BACKUP_SSH_HOST/USER/KEY/PORT      - if set, SCP upload after dump
#   BACKUP_REMOTE_DIR                  - remote target dir (default: ~/backups/)

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }

DB_USER="${DB_USER:-dev}"
DB_NAME="${DB_NAME:-tokobangunan}"
CONTAINER="${DB_CONTAINER:-tokobangunan-db}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETAIN_DAYS="${BACKUP_RETAIN_DAYS:-30}"
RETAIN_COUNT="${BACKUP_RETAIN_COUNT:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

# 1. Dump (+ optional encrypt) ------------------------------------------------
if [[ -n "${BACKUP_PASSPHRASE:-}" ]]; then
    OUT="$BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump.gz.gpg"
    PATTERN="tokobangunan_*.dump.gz.gpg"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F custom \
        | gzip \
        | gpg --batch --yes --quiet \
              --passphrase-file <(printf '%s' "$BACKUP_PASSPHRASE") \
              --symmetric --cipher-algo AES256 \
              -o "$OUT"
    log "encrypted backup ok: $OUT ($(du -h "$OUT" | cut -f1))"
else
    OUT="$BACKUP_DIR/tokobangunan_${TIMESTAMP}.dump"
    PATTERN="tokobangunan_*.dump"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -F custom \
        > "$OUT"
    log "backup ok: $OUT ($(du -h "$OUT" | cut -f1))"
fi

# 2. Optional SCP upload ------------------------------------------------------
if [[ -n "${BACKUP_SSH_HOST:-}" && -n "${BACKUP_SSH_USER:-}" && -n "${BACKUP_SSH_KEY:-}" ]]; then
    PORT="${BACKUP_SSH_PORT:-22}"
    REMOTE="${BACKUP_REMOTE_DIR:-~/backups/}"
    # Expand ~ in key path manually (shell does not in quoted vars).
    KEY="${BACKUP_SSH_KEY/#\~/$HOME}"
    if scp -i "$KEY" -P "$PORT" \
           -o StrictHostKeyChecking=accept-new \
           -o ConnectTimeout=20 \
           "$OUT" "${BACKUP_SSH_USER}@${BACKUP_SSH_HOST}:${REMOTE}"; then
        log "uploaded to ${BACKUP_SSH_USER}@${BACKUP_SSH_HOST}:${REMOTE}"
    else
        log "WARN: scp upload failed (local backup retained)"
    fi
else
    log "skip scp: BACKUP_SSH_* not fully set"
fi

# 3. Retention ----------------------------------------------------------------
# 3a. Prune by age.
find "$BACKUP_DIR" -maxdepth 1 -name "$PATTERN" -type f -mtime +"$RETAIN_DAYS" -print -delete \
    | sed "s|^|[$(date '+%Y-%m-%d %H:%M:%S')] pruned (age): |" || true

# 3b. Prune by count (keep newest N).
# shellcheck disable=SC2012
ls -1t "$BACKUP_DIR"/$PATTERN 2>/dev/null | tail -n +"$((RETAIN_COUNT + 1))" | while read -r f; do
    rm -f -- "$f"
    log "pruned (count): $f"
done

log "retention done (>${RETAIN_DAYS}d, keep ${RETAIN_COUNT})"

#!/usr/bin/env bash
set -euo pipefail

# Install daily backup cron at 02:00 local time via wrapper backup-cron.sh.
# Wrapper meng-source .env.backup / .env supaya BACKUP_PASSPHRASE & BACKUP_SSH_*
# tersedia di environment cron (cron tidak inherit user shell env).
#
# Idempotent: replaces any existing line referencing backup-cron.sh atau
# legacy backup.sh.
#
# Usage:
#   bash scripts/install-cron.sh           # install
#   CRON_TIME="0 3 * * *" bash scripts/install-cron.sh
#   bash scripts/install-cron.sh --remove  # uninstall

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WRAPPER="$PROJECT_DIR/scripts/backup-cron.sh"
LOG_FILE="${BACKUP_LOG_FILE:-$PROJECT_DIR/backups/backup.log}"
CRON_TIME="${CRON_TIME:-0 2 * * *}"
SCHEDULE_LINE="$CRON_TIME bash $WRAPPER >> $LOG_FILE 2>&1"

mkdir -p "$(dirname "$LOG_FILE")"

# Match either the new wrapper OR legacy backup.sh entries when filtering.
FILTER='tokobangunan/scripts/backup'

if [[ "${1:-}" == "--remove" ]]; then
    crontab -l 2>/dev/null \
        | grep -v "$FILTER" \
        | crontab -
    echo "Cron removed."
    exit 0
fi

# Pull existing crontab (ignore errors if empty), strip prior tokobangunan
# backup line, append the new one, then install.
{
    crontab -l 2>/dev/null | grep -v "$FILTER" || true
    echo "$SCHEDULE_LINE"
} | crontab -

echo "Cron installed:"
echo "  $SCHEDULE_LINE"
echo
echo "Verify:    crontab -l"
echo "Log file:  $LOG_FILE"
echo "Remove:    bash scripts/install-cron.sh --remove"

#!/usr/bin/env bash
set -euo pipefail

# Install daily backup cron at 02:00 local time.
# Idempotent: replaces any existing line referencing tokobangunan/scripts/backup.sh.
#
# Usage:
#   bash scripts/install-cron.sh           # install
#   CRON_TIME="0 3 * * *" bash scripts/install-cron.sh
#   bash scripts/install-cron.sh --remove  # uninstall

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LOG_FILE="${BACKUP_LOG_FILE:-$PROJECT_DIR/backups/backup.log}"
CRON_TIME="${CRON_TIME:-0 2 * * *}"
SCHEDULE_LINE="$CRON_TIME cd $PROJECT_DIR && bash scripts/backup.sh >> $LOG_FILE 2>&1"

mkdir -p "$(dirname "$LOG_FILE")"

if [[ "${1:-}" == "--remove" ]]; then
    crontab -l 2>/dev/null \
        | grep -v 'tokobangunan/scripts/backup.sh' \
        | crontab -
    echo "Cron removed."
    exit 0
fi

# Pull existing crontab (ignore errors if empty), strip prior tokobangunan
# backup line, append the new one, then install.
{
    crontab -l 2>/dev/null | grep -v 'tokobangunan/scripts/backup.sh' || true
    echo "$SCHEDULE_LINE"
} | crontab -

echo "Cron installed:"
echo "  $SCHEDULE_LINE"
echo
echo "Verify:    crontab -l"
echo "Log file:  $LOG_FILE"
echo "Remove:    bash scripts/install-cron.sh --remove"

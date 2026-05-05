#!/usr/bin/env bash
# Wrapper untuk cron — load .env lalu jalankan backup.sh.
#
# Cron tidak inherit shell env, jadi BACKUP_PASSPHRASE / BACKUP_SSH_* harus
# di-load eksplisit dari file. Prefer .env.backup (dedicated, chmod 600);
# fallback ke .env kalau tidak ada.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

for envfile in .env.backup .env; do
    if [[ -f "$envfile" ]]; then
        set -a
        # shellcheck disable=SC1090
        source "$envfile"
        set +a
        break
    fi
done

bash scripts/backup.sh

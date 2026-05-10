#!/usr/bin/env bash
# Generate kuat secrets untuk .env.backup
set -euo pipefail
cat <<EOF
# Generated $(date)
# Copy ke .env.backup, chmod 600.

# Backup encryption — JANGAN HILANG, tanpa ini backup tidak bisa di-restore
BACKUP_PASSPHRASE=$(openssl rand -base64 32)

# Hostinger SCP destination
BACKUP_SSH_HOST=185.214.124.85
BACKUP_SSH_PORT=65002
BACKUP_SSH_USER=u212852160
BACKUP_SSH_KEY=$HOME/.ssh/hostinger_scriptsis
BACKUP_REMOTE_DIR=~/backups/tokobangunan/

# Retention
BACKUP_RETAIN_DAYS=30
BACKUP_RETAIN_COUNT=30
EOF

#!/usr/bin/env bash
# partition-rollover.sh
#
# Auto-add penjualan partition for next year if it does not exist yet.
# Safe to run repeatedly (idempotent). Intended for cron on 1 January.
#
# Usage:
#   bash scripts/partition-rollover.sh
#
# Cron example (run 00:00 every 1 January):
#   0 0 1 1 * cd /path/to/tokobangunan && bash scripts/partition-rollover.sh >> /var/log/penjualan-partition.log 2>&1

set -euo pipefail

CONTAINER="${TOKOBANGUNAN_DB_CONTAINER:-tokobangunan-db}"
DB_USER="${TOKOBANGUNAN_DB_USER:-dev}"
DB_NAME="${TOKOBANGUNAN_DB_NAME:-tokobangunan}"

NEXT_YEAR=$(($(date +%Y) + 1))
START="${NEXT_YEAR}-01-01"
END="$((NEXT_YEAR + 1))-01-01"
TABLE="penjualan_${NEXT_YEAR}"

EXISTS=$(docker exec "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -tAc \
    "SELECT 1 FROM pg_class WHERE relname='${TABLE}';")

if [[ "${EXISTS}" == "1" ]]; then
    echo "[$(date -Iseconds)] ${TABLE} already exists, skip"
    exit 0
fi

docker exec "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c \
    "CREATE TABLE ${TABLE} PARTITION OF penjualan FOR VALUES FROM ('${START}') TO ('${END}');"

echo "[$(date -Iseconds)] created ${TABLE}"

#!/usr/bin/env bash
# Generate kredensial basic-auth untuk endpoint /metrics.
# Copy output ke .env (atau secret store) lalu chmod 600.
set -euo pipefail
echo "METRICS_USER=tb_metrics_$(openssl rand -hex 4)"
echo "METRICS_PASS=$(openssl rand -base64 24)"

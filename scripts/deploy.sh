#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Quiz App - Deploy to VPS
# ============================================================
# Usage:
#   ./deploy.sh user@your-vps-ip
#
# Prerequisites on VPS:
#   - nginx installed and running
#   - systemd
#
# What this script does:
#   1. Cross-compiles the binary for Linux amd64
#   2. Copies binary + quiz.db to VPS
#   3. Sets up systemd service
#   4. Prints nginx config snippet to add manually
# ============================================================

REMOTE="${1:?Usage: ./deploy.sh user@host}"
APP_NAME="quiz"
REMOTE_DIR="/opt/quiz"
SERVICE_NAME="quiz"
BASE_PATH="/exams"
PORT="8081"

echo "==> Building for linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${APP_NAME}-linux" .

echo "==> Uploading to ${REMOTE}:${REMOTE_DIR}..."
ssh "$REMOTE" "sudo mkdir -p ${REMOTE_DIR} && sudo chown \$(whoami) ${REMOTE_DIR}"
scp "${APP_NAME}-linux" "$REMOTE:${REMOTE_DIR}/${APP_NAME}"

##### DANGEROUS: This will overwrite the existing quiz.db on the server. Make sure you have a backup before running this!
# scp quiz.db "$REMOTE:${REMOTE_DIR}/quiz.db"
#####

ssh "$REMOTE" "chmod +x ${REMOTE_DIR}/${APP_NAME}"

echo "==> Setting up systemd service..."
ssh "$REMOTE" "sudo tee /etc/systemd/system/${SERVICE_NAME}.service > /dev/null" <<EOF
[Unit]
Description=Quiz App
After=network.target

[Service]
Type=simple
WorkingDirectory=${REMOTE_DIR}
ExecStart=${REMOTE_DIR}/${APP_NAME}
Environment=PORT=${PORT}
Environment=BASE_PATH=${BASE_PATH}
Environment=DB_PATH=${REMOTE_DIR}/quiz.db
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

ssh "$REMOTE" "sudo systemctl daemon-reload && sudo systemctl enable ${SERVICE_NAME} && sudo systemctl restart ${SERVICE_NAME}"

echo "==> Checking service status..."
ssh "$REMOTE" "sudo systemctl status ${SERVICE_NAME} --no-pager -l" || true

rm -f "${APP_NAME}-linux"

echo ""
echo "==> Done! App is running at ${REMOTE}:${PORT}${BASE_PATH}/"
echo ""
echo "==> Add this to your nginx config (e.g. /etc/nginx/sites-available/default):"
echo ""
cat <<'NGINX'
    location /exams/ {
        proxy_pass http://127.0.0.1:8081/exams/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
NGINX
echo ""
echo "Then run: sudo nginx -t && sudo systemctl reload nginx"

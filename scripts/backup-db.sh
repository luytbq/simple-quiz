#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Quiz App - Backup DB from VPS
# ============================================================
# Usage:
#   ./scripts/backup-db.sh user@your-vps-ip
#
# Downloads quiz.db from VPS to ./backups/quiz_YYYYMMDD_HHMMSS.db
# Also overwrites ./quiz.db with the latest backup.
# ============================================================

REMOTE="${1:?Usage: ./scripts/backup-db.sh user@host}"
REMOTE_DIR="/opt/quiz"
BACKUP_DIR="$(cd "$(dirname "$0")/.." && pwd)/backups"

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/quiz_${TIMESTAMP}.db"

echo "==> Downloading quiz.db from ${REMOTE}..."
scp "$REMOTE:${REMOTE_DIR}/quiz.db" "$BACKUP_FILE"

# Also copy as the working quiz.db
#cp "$BACKUP_FILE" "$(dirname "$0")/../quiz.db"

echo "==> Saved to ${BACKUP_FILE}"
echo "==> Updated ./quiz.db"
echo ""

# Show recent backups
echo "Recent backups:"
ls -lht "$BACKUP_DIR" | head -6

#!/bin/bash
# PostgreSQL Backup Script
# Requirements: 7.3
#
# This script performs a full PostgreSQL backup using pg_dump with custom format.
# It supports parallel restore and compression.
#
# Usage:
#   ./scripts/backup/backup.sh
#
# Environment Variables:
#   DB_URL              - PostgreSQL connection string (required)
#   BACKUP_DIR          - Directory to store backups (default: /backups/full)
#   PGCONNECT_TIMEOUT   - Connection timeout in seconds (default: 30)
#
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="${BACKUP_DIR:-/backups/full}"
BACKUP_FILE="${BACKUP_DIR}/auction_db_${TIMESTAMP}.dump"
METADATA_FILE="${BACKUP_DIR}/auction_db_${TIMESTAMP}.meta.json"
export PGCONNECT_TIMEOUT="${PGCONNECT_TIMEOUT:-30}"

# Validate required environment variables
if [ -z "${DB_URL:-}" ]; then
    echo "[$(date -Iseconds)] ERROR: DB_URL environment variable is required"
    exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Ensure backup directory exists
# ─────────────────────────────────────────────────────────────────────────────
mkdir -p "${BACKUP_DIR}"

echo "============================================================"
echo "[$(date -Iseconds)] PostgreSQL Backup Starting"
echo "============================================================"
echo "  Output file: ${BACKUP_FILE}"
echo "  Format: custom (compressed, parallel-restore capable)"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Execute pg_dump
# ─────────────────────────────────────────────────────────────────────────────
START_TIME=$(date +%s)

echo "[$(date -Iseconds)] Running pg_dump..."
pg_dump \
    --dbname="$DB_URL" \
    --format=custom \
    --compress=6 \
    --verbose \
    --no-password \
    --file="$BACKUP_FILE" \
    2>&1 | tee /tmp/backup.log

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# ─────────────────────────────────────────────────────────────────────────────
# Verify backup
# ─────────────────────────────────────────────────────────────────────────────
if [ ! -f "$BACKUP_FILE" ] || [ ! -s "$BACKUP_FILE" ]; then
    echo "[$(date -Iseconds)] ERROR: Backup file is missing or empty!"
    exit 1
fi

BACKUP_SIZE=$(stat -f%z "$BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_FILE" 2>/dev/null || echo "0")
BACKUP_SIZE_HUMAN=$(du -h "$BACKUP_FILE" | cut -f1)

# ─────────────────────────────────────────────────────────────────────────────
# Collect table metadata for verification
# ─────────────────────────────────────────────────────────────────────────────
echo "[$(date -Iseconds)] Collecting table metadata..."
psql "$DB_URL" --no-password -t -A -F'|' -c "
    SELECT schemaname || '.' || relname AS table_name,
           n_live_tup AS row_count
    FROM pg_stat_user_tables
    ORDER BY schemaname, relname;
" > /tmp/table_counts.txt

# Generate metadata JSON
TABLE_JSON=""
while IFS='|' read -r name count; do
    [ -z "$name" ] && continue
    if [ -n "$TABLE_JSON" ]; then
        TABLE_JSON="${TABLE_JSON},"
    fi
    TABLE_JSON="${TABLE_JSON}{\"name\":\"${name}\",\"row_count\":${count:-0}}"
done < /tmp/table_counts.txt

cat > "$METADATA_FILE" << EOF
{
  "backup_file": "$(basename $BACKUP_FILE)",
  "timestamp": "$(date -Iseconds)",
  "duration_seconds": ${DURATION},
  "size_bytes": ${BACKUP_SIZE},
  "size_human": "${BACKUP_SIZE_HUMAN}",
  "database": "auction_db",
  "tables": [${TABLE_JSON}],
  "pg_version": "$(pg_dump --version | head -1)"
}
EOF

# ─────────────────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────────────────
TABLE_COUNT=$(wc -l < /tmp/table_counts.txt | tr -d ' ')

echo ""
echo "============================================================"
echo "BACKUP COMPLETE"
echo "============================================================"
echo "  File:     ${BACKUP_FILE}"
echo "  Size:     ${BACKUP_SIZE_HUMAN}"
echo "  Duration: ${DURATION}s"
echo "  Tables:   ${TABLE_COUNT}"
echo "  Metadata: ${METADATA_FILE}"
echo "============================================================"
echo "[$(date -Iseconds)] ✅ Backup successful"

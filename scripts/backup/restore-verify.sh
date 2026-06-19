#!/bin/bash
# PostgreSQL Restore Verification Script
# Requirement 7.4:
#   Verify backup integrity by performing automated restore test.
#   Success = completed restore to temp DB + all tables/row counts match within 1% variance.
#
# Usage:
#   ./scripts/backup/restore-verify.sh
#
# Environment Variables:
#   DB_URL              - Source database connection string (required)
#   RESTORE_DB_HOST     - Host for temporary restore database (required)
#   RESTORE_DB_PORT     - Port for restore database (default: 5432)
#   RESTORE_DB_USER     - Username for restore database (required)
#   RESTORE_DB_PASSWORD - Password for restore database (required)
#   BACKUP_DIR          - Directory containing backups (default: /backups/full)
#   MAX_VARIANCE        - Maximum allowed row count variance (default: 0.01 = 1%)
#
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────
BACKUP_DIR="${BACKUP_DIR:-/backups/full}"
MAX_VARIANCE="${MAX_VARIANCE:-0.01}"
RESTORE_DB_PORT="${RESTORE_DB_PORT:-5432}"
TEMP_DB="auction_restore_verify_$(date +%s)"

# Validate required environment variables
for var in DB_URL RESTORE_DB_HOST RESTORE_DB_USER RESTORE_DB_PASSWORD; do
    if [ -z "${!var:-}" ]; then
        echo "[$(date -Iseconds)] ERROR: ${var} environment variable is required"
        exit 1
    fi
done

RESTORE_URL="postgresql://${RESTORE_DB_USER}:${RESTORE_DB_PASSWORD}@${RESTORE_DB_HOST}:${RESTORE_DB_PORT}/postgres"
TEMP_DB_URL="postgresql://${RESTORE_DB_USER}:${RESTORE_DB_PASSWORD}@${RESTORE_DB_HOST}:${RESTORE_DB_PORT}/${TEMP_DB}"

echo "============================================================"
echo "[$(date -Iseconds)] PostgreSQL Restore Verification Starting"
echo "============================================================"
echo "  Backup dir:    ${BACKUP_DIR}"
echo "  Max variance:  ${MAX_VARIANCE} ($(awk "BEGIN{printf \"%.0f\", $MAX_VARIANCE * 100}")%)"
echo "  Restore host:  ${RESTORE_DB_HOST}:${RESTORE_DB_PORT}"
echo "  Temp database: ${TEMP_DB}"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Find latest backup
# ─────────────────────────────────────────────────────────────────────────────
LATEST_BACKUP=$(ls -t "${BACKUP_DIR}"/*.dump 2>/dev/null | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo "[$(date -Iseconds)] ERROR: No backup files found in ${BACKUP_DIR}"
    exit 1
fi

echo "[$(date -Iseconds)] Latest backup: $(basename $LATEST_BACKUP)"
echo "[$(date -Iseconds)] Backup size: $(du -h $LATEST_BACKUP | cut -f1)"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Cleanup function (always drop temp database on exit)
# ─────────────────────────────────────────────────────────────────────────────
cleanup() {
    echo ""
    echo "[$(date -Iseconds)] Cleaning up temporary database: ${TEMP_DB}"
    psql "$RESTORE_URL" --no-password -c "DROP DATABASE IF EXISTS ${TEMP_DB};" 2>/dev/null || true
}
trap cleanup EXIT

# ─────────────────────────────────────────────────────────────────────────────
# Create temporary database
# ─────────────────────────────────────────────────────────────────────────────
echo "[$(date -Iseconds)] Creating temporary database..."
psql "$RESTORE_URL" --no-password -c "CREATE DATABASE ${TEMP_DB};"

# ─────────────────────────────────────────────────────────────────────────────
# Restore backup
# ─────────────────────────────────────────────────────────────────────────────
echo "[$(date -Iseconds)] Restoring backup to temporary database..."
START_TIME=$(date +%s)

pg_restore \
    --dbname="$TEMP_DB_URL" \
    --no-owner \
    --no-privileges \
    --jobs=4 \
    --verbose \
    "$LATEST_BACKUP" 2>&1 | tail -20

END_TIME=$(date +%s)
RESTORE_DURATION=$((END_TIME - START_TIME))
echo "[$(date -Iseconds)] Restore completed in ${RESTORE_DURATION}s"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Validate tables
# ─────────────────────────────────────────────────────────────────────────────
echo "[$(date -Iseconds)] Validating tables..."

SOURCE_TABLE_COUNT=$(psql "$DB_URL" --no-password -t -A -c "
    SELECT count(*) FROM pg_tables
    WHERE schemaname NOT IN ('pg_catalog', 'information_schema');
")

RESTORED_TABLE_COUNT=$(psql "$TEMP_DB_URL" --no-password -t -A -c "
    SELECT count(*) FROM pg_tables
    WHERE schemaname NOT IN ('pg_catalog', 'information_schema');
")

SOURCE_TABLE_COUNT=$(echo "$SOURCE_TABLE_COUNT" | tr -d ' ')
RESTORED_TABLE_COUNT=$(echo "$RESTORED_TABLE_COUNT" | tr -d ' ')

echo "  Source tables:   ${SOURCE_TABLE_COUNT}"
echo "  Restored tables: ${RESTORED_TABLE_COUNT}"

if [ "$SOURCE_TABLE_COUNT" -ne "$RESTORED_TABLE_COUNT" ]; then
    echo "[$(date -Iseconds)] ERROR: Table count mismatch!"
    exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Validate row counts within variance threshold
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo "[$(date -Iseconds)] Validating row counts (max variance: ${MAX_VARIANCE})..."
echo "─────────────────────────────────────────────────────────────"

VERIFICATION_PASSED=true
TABLES_CHECKED=0
TABLES_FAILED=0

# Run ANALYZE on restored DB for accurate pg_stat counts
psql "$TEMP_DB_URL" --no-password -c "ANALYZE;" 2>/dev/null

# Get row counts from source
psql "$DB_URL" --no-password -t -A -F'|' -c "
    SELECT schemaname || '.' || relname, n_live_tup
    FROM pg_stat_user_tables
    ORDER BY schemaname, relname;
" > /tmp/source_counts.txt

while IFS='|' read -r TABLE_NAME SOURCE_COUNT; do
    [ -z "$TABLE_NAME" ] && continue
    TABLES_CHECKED=$((TABLES_CHECKED + 1))

    SOURCE_COUNT=$(echo "$SOURCE_COUNT" | tr -d ' ')
    [ -z "$SOURCE_COUNT" ] && SOURCE_COUNT=0

    # Get restored count
    RESTORED_COUNT=$(psql "$TEMP_DB_URL" --no-password -t -A -c "
        SELECT n_live_tup FROM pg_stat_user_tables
        WHERE schemaname || '.' || relname = '${TABLE_NAME}';
    " 2>/dev/null | tr -d ' ')
    [ -z "$RESTORED_COUNT" ] && RESTORED_COUNT=0

    # Calculate variance
    if [ "$SOURCE_COUNT" -eq 0 ] && [ "$RESTORED_COUNT" -eq 0 ]; then
        VARIANCE="0.0000"
        STATUS="✅"
    elif [ "$SOURCE_COUNT" -eq 0 ]; then
        VARIANCE="1.0000"
        STATUS="❌"
        VERIFICATION_PASSED=false
        TABLES_FAILED=$((TABLES_FAILED + 1))
    else
        VARIANCE=$(awk "BEGIN {
            diff = ($RESTORED_COUNT - $SOURCE_COUNT);
            if (diff < 0) diff = -diff;
            printf \"%.4f\", diff / $SOURCE_COUNT
        }")

        EXCEEDS=$(awk "BEGIN { print ($VARIANCE > $MAX_VARIANCE) ? \"yes\" : \"no\" }")
        if [ "$EXCEEDS" = "yes" ]; then
            STATUS="❌"
            VERIFICATION_PASSED=false
            TABLES_FAILED=$((TABLES_FAILED + 1))
        else
            STATUS="✅"
        fi
    fi

    printf "  %s %-40s source=%-8s restored=%-8s variance=%s\n" \
        "$STATUS" "$TABLE_NAME" "$SOURCE_COUNT" "$RESTORED_COUNT" "$VARIANCE"

done < /tmp/source_counts.txt

# ─────────────────────────────────────────────────────────────────────────────
# Report
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo "============================================================"
echo "RESTORE VERIFICATION REPORT"
echo "============================================================"
echo "  Backup file:      $(basename $LATEST_BACKUP)"
echo "  Restore duration: ${RESTORE_DURATION}s"
echo "  Tables checked:   ${TABLES_CHECKED}"
echo "  Tables failed:    ${TABLES_FAILED}"
echo "  Max variance:     ${MAX_VARIANCE}"
echo "============================================================"

if [ "$VERIFICATION_PASSED" = "true" ]; then
    echo "[$(date -Iseconds)] ✅ RESTORE VERIFICATION PASSED"
    exit 0
else
    echo "[$(date -Iseconds)] ❌ RESTORE VERIFICATION FAILED"
    echo "  ${TABLES_FAILED} table(s) exceed the allowed ${MAX_VARIANCE} variance threshold."
    exit 1
fi

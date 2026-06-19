#!/bin/bash
# PostgreSQL Backup Retention Cleanup Script
# Requirement 7.6:
#   Retain backups min 30 days, max 90 days.
#   Auto-delete backups beyond max retention period.
#
# Usage:
#   ./scripts/backup/retention-cleanup.sh
#
# Environment Variables:
#   BACKUP_DIR          - Directory containing backups (default: /backups/full)
#   WAL_DIR             - Directory containing WAL archives (default: /backups/wal)
#   MIN_RETENTION_DAYS  - Minimum days to keep backups (default: 30)
#   MAX_RETENTION_DAYS  - Maximum days to keep backups (default: 90)
#   WAL_RETENTION_DAYS  - Days to keep WAL files (default: 7)
#   DRY_RUN             - If "true", only report what would be deleted (default: false)
#
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────
BACKUP_DIR="${BACKUP_DIR:-/backups/full}"
WAL_DIR="${WAL_DIR:-/backups/wal}"
MIN_RETENTION_DAYS="${MIN_RETENTION_DAYS:-30}"
MAX_RETENTION_DAYS="${MAX_RETENTION_DAYS:-90}"
WAL_RETENTION_DAYS="${WAL_RETENTION_DAYS:-7}"
DRY_RUN="${DRY_RUN:-false}"

echo "============================================================"
echo "[$(date -Iseconds)] Backup Retention Cleanup"
echo "============================================================"
echo "  Backup dir:         ${BACKUP_DIR}"
echo "  WAL dir:            ${WAL_DIR}"
echo "  Min retention:      ${MIN_RETENTION_DAYS} days"
echo "  Max retention:      ${MAX_RETENTION_DAYS} days"
echo "  WAL retention:      ${WAL_RETENTION_DAYS} days"
echo "  Dry run:            ${DRY_RUN}"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Cleanup full backups beyond max retention
# ─────────────────────────────────────────────────────────────────────────────
echo "[$(date -Iseconds)] Scanning for backups older than ${MAX_RETENTION_DAYS} days..."

DELETED_COUNT=0
DELETED_SIZE=0

if [ -d "$BACKUP_DIR" ]; then
    while IFS= read -r -d '' file; do
        FILE_SIZE=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0")
        
        if [ "$DRY_RUN" = "true" ]; then
            echo "  [DRY RUN] Would delete: $(basename $file) ($(du -h "$file" | cut -f1))"
        else
            echo "  Deleting: $(basename $file) ($(du -h "$file" | cut -f1))"
            rm -f "$file"
            
            # Also remove associated metadata file
            META_FILE="${file%.dump}.meta.json"
            [ -f "$META_FILE" ] && rm -f "$META_FILE"
        fi
        
        DELETED_COUNT=$((DELETED_COUNT + 1))
        DELETED_SIZE=$((DELETED_SIZE + FILE_SIZE))
    done < <(find "$BACKUP_DIR" -name "*.dump" -type f -mtime +${MAX_RETENTION_DAYS} -print0 2>/dev/null)
fi

# ─────────────────────────────────────────────────────────────────────────────
# Cleanup WAL files beyond WAL retention
# ─────────────────────────────────────────────────────────────────────────────
WAL_DELETED=0

if [ -d "$WAL_DIR" ]; then
    echo ""
    echo "[$(date -Iseconds)] Scanning for WAL files older than ${WAL_RETENTION_DAYS} days..."
    
    while IFS= read -r -d '' file; do
        if [ "$DRY_RUN" = "true" ]; then
            echo "  [DRY RUN] Would delete: $(basename $file)"
        else
            rm -f "$file"
        fi
        WAL_DELETED=$((WAL_DELETED + 1))
    done < <(find "$WAL_DIR" -type f -mtime +${WAL_RETENTION_DAYS} -print0 2>/dev/null)
fi

# ─────────────────────────────────────────────────────────────────────────────
# Cleanup orphaned metadata files
# ─────────────────────────────────────────────────────────────────────────────
ORPHANED=0

if [ -d "$BACKUP_DIR" ]; then
    for meta in "$BACKUP_DIR"/*.meta.json; do
        [ -f "$meta" ] || continue
        DUMP_FILE="${meta%.meta.json}.dump"
        if [ ! -f "$DUMP_FILE" ]; then
            if [ "$DRY_RUN" = "true" ]; then
                echo "  [DRY RUN] Would remove orphan: $(basename $meta)"
            else
                rm -f "$meta"
            fi
            ORPHANED=$((ORPHANED + 1))
        fi
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
# Report
# ─────────────────────────────────────────────────────────────────────────────
REMAINING=$(find "$BACKUP_DIR" -name "*.dump" -type f 2>/dev/null | wc -l | tr -d ' ')
DELETED_SIZE_HUMAN=$(awk "BEGIN { printf \"%.2f MB\", $DELETED_SIZE / 1048576 }")

echo ""
echo "============================================================"
echo "RETENTION CLEANUP REPORT"
echo "============================================================"
echo "  Backups deleted:    ${DELETED_COUNT} (${DELETED_SIZE_HUMAN})"
echo "  WAL files deleted:  ${WAL_DELETED}"
echo "  Orphaned metadata:  ${ORPHANED}"
echo "  Remaining backups:  ${REMAINING}"

if [ -d "$BACKUP_DIR" ]; then
    BACKUP_USAGE=$(du -sh "$BACKUP_DIR" 2>/dev/null | cut -f1 || echo "N/A")
    echo "  Backup disk usage:  ${BACKUP_USAGE}"
fi
if [ -d "$WAL_DIR" ]; then
    WAL_USAGE=$(du -sh "$WAL_DIR" 2>/dev/null | cut -f1 || echo "N/A")
    echo "  WAL disk usage:     ${WAL_USAGE}"
fi

echo "============================================================"

# Safety check
if [ "$REMAINING" -lt 4 ] && [ "$DELETED_COUNT" -gt 0 ]; then
    echo "[$(date -Iseconds)] ⚠️  WARNING: Very few backups remaining (${REMAINING})."
    echo "  Verify that backup CronJob is running on schedule."
fi

echo "[$(date -Iseconds)] ✅ Retention cleanup complete"

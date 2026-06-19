# PostgreSQL Backup & Restore Strategy

## Overview

Automated backup and restore verification system for the auction-system PostgreSQL database.

**Requirements covered:** 7.3, 7.4, 7.6, 7.7

## Components

| File | Purpose | Schedule |
|------|---------|----------|
| `postgres-backup-cronjob.yaml` | Full pg_dump backup | Every 6 hours |
| `wal-archiving-config.yaml` | WAL archiving for PITR + daily WAL cleanup | Daily at 03:00 UTC |
| `restore-verification-cronjob.yaml` | Automated restore test with validation | Weekly (Sunday 04:00 UTC) |
| `retention-cleanup-cronjob.yaml` | Delete backups beyond max retention | Daily at 02:00 UTC |
| `backup-alerting-rules.yaml` | Prometheus alerting rules for DBA | Continuous |

## Backup Strategy

### Full Backups (Requirement 7.3)
- **Frequency:** Every 6 hours (00:00, 06:00, 12:00, 18:00 UTC)
- **Format:** pg_dump custom format (compressed, parallel-restore capable)
- **Timeout:** 60 minutes (activeDeadlineSeconds: 3600)
- **Retry:** 1 retry on failure (Requirement 7.7)

### WAL Archiving (Requirement 7.3)
- **Purpose:** Point-in-time recovery (PITR)
- **Retention:** Minimum 7 days
- **Archive command:** Copies WAL segments to backup PVC + gzip compression
- **Cleanup:** Daily removal of WAL files older than 7 days

### Restore Verification (Requirement 7.4)
- **Frequency:** Weekly (Sunday 04:00 UTC)
- **Process:**
  1. Find most recent backup file
  2. Create temporary database on restore host
  3. Restore backup using pg_restore (parallel, 4 jobs)
  4. Compare table count: source vs restored
  5. Compare row counts per table: max 1% variance allowed
  6. Drop temporary database (cleanup on exit)
- **Success criteria:** All tables present + all row counts within 1% variance

### Retention Policy (Requirement 7.6)
- **Minimum retention:** 30 days
- **Maximum retention:** 90 days
- **Auto-delete:** Backups older than 90 days are automatically removed daily
- **WAL retention:** 7 days

### Alerting (Requirement 7.7)
- **PostgresBackupFailed:** Fires when backup job fails/times out → alerts DBA channel
- **PostgresBackupMissing:** No successful backup in 12+ hours → critical alert
- **PostgresBackupSlow:** Backup running > 45 minutes → warning (approaching 60-min timeout)
- **PostgresRestoreVerificationFailed:** Weekly restore test failed → critical alert
- **PostgresBackupStorageLow:** Backup PVC < 15% available → warning
- **PostgresRetentionCleanupMissing:** Cleanup not run in 48+ hours → warning

## Secrets Required

The following secrets must exist in `auction-system-secrets`:

| Key | Description |
|-----|-------------|
| `DB_URL` | Source PostgreSQL connection string |
| `RESTORE_DB_HOST` | Host for temporary restore database |
| `RESTORE_DB_USER` | Username for restore database |
| `RESTORE_DB_PASSWORD` | Password for restore database |

## Manual Scripts

Located in `scripts/backup/`:

```bash
# Run manual backup
export DB_URL="postgresql://user:pass@host:5432/auction_db"
./scripts/backup/backup.sh

# Run manual restore verification
export RESTORE_DB_HOST="restore-host"
export RESTORE_DB_USER="admin"
export RESTORE_DB_PASSWORD="secret"
./scripts/backup/restore-verify.sh

# Run retention cleanup (dry run)
export DRY_RUN=true
./scripts/backup/retention-cleanup.sh
```

## Storage

- **PVC:** `postgres-backup-pvc` (200Gi, ReadWriteOnce)
- **Structure:**
  ```
  /backups/
  ├── full/
  │   ├── auction_db_20240115_060000.dump
  │   ├── auction_db_20240115_060000.meta.json
  │   ├── auction_db_20240115_120000.dump
  │   └── ...
  └── wal/
      ├── 000000010000000100000042.gz
      └── ...
  ```

## Deployment

```bash
# Apply all backup manifests
kubectl apply -f deployments/k8s/backup/

# Apply alerting rules
kubectl apply -f deployments/k8s/backup/backup-alerting-rules.yaml

# Verify CronJobs are created
kubectl get cronjobs -n auction-system | grep postgres

# Trigger manual backup
kubectl create job --from=cronjob/postgres-backup manual-backup-$(date +%s) -n auction-system
```

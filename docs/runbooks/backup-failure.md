# Runbook: Backup Failure

## Alert Description

**Alert Name:** BackupFailure  
**Severity:** Critical  
**Trigger Condition:** PostgreSQL backup Job fails to complete within 60 minutes or exits with error  
**Prometheus Expression:** `backup_job_duration_seconds > 3600` OR `backup_job_status == 0` (failure)

## Symptoms

- Backup CronJob reports failure status in Kubernetes
- No new backup artifacts in configured storage within the 6-hour schedule
- Automated restore test fails during weekly verification
- Alert notification sent to DBA channel
- Potential RPO (Recovery Point Objective) violation if multiple backups fail

## Diagnosis Steps

### Step 1: Check Backup Job Status

```bash
# List recent backup jobs
kubectl get jobs -n auction-system -l app=postgres-backup --sort-by='.status.startTime'

# Describe the failed job
kubectl describe job <backup-job-name> -n auction-system

# Check job pod logs
kubectl logs job/<backup-job-name> -n auction-system
```

Look for:
- Connection refused errors (database unreachable)
- Permission denied (credentials issue)
- Disk space errors (storage full)
- Timeout (backup too large for window)

### Step 2: Check Database Health

```bash
# Verify PostgreSQL primary is accessible
kubectl exec -n auction-system <db-pod> -- pg_isready

# Check WAL archiving status
kubectl exec -n auction-system <db-pod> -- psql -c "SELECT * FROM pg_stat_archiver;"

# Check database size (large DB may exceed backup window)
kubectl exec -n auction-system <db-pod> -- psql -c "SELECT pg_size_pretty(pg_database_size('auction'));"
```

### Step 3: Verify Storage Backend

```bash
# Check backup storage available space
kubectl exec -n auction-system <backup-pod> -- df -h /backup-storage

# List existing backups
kubectl exec -n auction-system <backup-pod> -- ls -la /backup-storage/

# Verify storage credentials (S3/GCS/Azure Blob)
kubectl get secret -n auction-system backup-storage-credentials -o jsonpath='{.data}' | base64 -d
```

### Step 4: Check WAL Archiving

```bash
# Verify WAL archiving is functioning
kubectl exec -n auction-system <db-pod> -- psql -c "SELECT last_archived_wal, last_archived_time, last_failed_wal, last_failed_time FROM pg_stat_archiver;"

# Check pg_wal directory size
kubectl exec -n auction-system <db-pod> -- du -sh /var/lib/postgresql/data/pg_wal/
```

If WAL archiving is failing, point-in-time recovery may be compromised.

### Step 5: Review Retention Policy

```bash
# Check backup count and oldest backup
kubectl exec -n auction-system <backup-pod> -- ls -lt /backup-storage/ | head -20

# Verify retention (30-90 days) is not causing storage pressure
kubectl exec -n auction-system <backup-pod> -- find /backup-storage/ -mtime +90 -type f | wc -l
```

## Resolution

### Immediate Actions

1. **Automatic retry:** The system will automatically retry the backup once within 15 minutes of failure. Monitor the retry job:
   ```bash
   kubectl get jobs -n auction-system -l app=postgres-backup --watch
   ```

2. **If storage is full:** Clean up old backups beyond retention policy or expand storage:
   ```bash
   # Delete backups older than max retention (90 days)
   kubectl exec -n auction-system <backup-pod> -- find /backup-storage/ -mtime +90 -delete
   ```

3. **If credentials expired:** Refresh storage backend credentials:
   ```bash
   # Check and update sealed secret
   kubectl get sealedsecrets -n auction-system backup-storage-credentials
   ```

4. **If database too large for backup window:** Consider:
   - Switching to incremental/differential backups
   - Increasing backup timeout
   - Running backup against replica instead of primary

5. **If WAL archiving broken:** Restart WAL archiver and verify:
   ```bash
   kubectl exec -n auction-system <db-pod> -- psql -c "SELECT pg_switch_wal();"
   # Verify new WAL is archived
   kubectl exec -n auction-system <db-pod> -- psql -c "SELECT * FROM pg_stat_archiver;"
   ```

### Post-Incident

- Verify next scheduled backup completes successfully
- Confirm point-in-time recovery capability is maintained
- Run manual restore test to validate backup integrity
- Review backup job resource limits (memory, CPU)
- Update backup strategy if database growth exceeds window

## Escalation

| Level | Contact | When |
|-------|---------|------|
| L1 | On-call SRE | Alert fires |
| L2 | DBA team | Automatic retry also fails |
| L3 | Platform engineering | Storage backend issues |
| L4 | Engineering manager | Multiple backup cycles missed (RPO at risk) |

**PagerDuty Service:** `auction-system-critical`  
**Slack Channel:** `#auction-dba`

## Important Notes

- **RPO Target:** 1 minute (via WAL archiving)
- **Backup Schedule:** Every 6 hours via pg_dump + continuous WAL archiving
- **Retention:** 30-90 days, auto-cleanup of backups beyond max retention
- **Weekly restore test:** Automated restore to temp DB with table/row validation (±1% variance)

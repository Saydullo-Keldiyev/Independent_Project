# Runbook: Pod Restart Loop

## Alert Description

**Alert Name:** PodRestartLoop  
**Severity:** Warning  
**Trigger Condition:** Pod restart count exceeds 3 within 10 minutes  
**Prometheus Expression:** `increase(kube_pod_container_status_restarts_total[10m]) > 3`

## Symptoms

- Service intermittently unavailable (flapping between healthy and unhealthy)
- Elevated error rates due to pods being in CrashLoopBackOff
- Kubernetes events showing repeated container restarts
- Health check failures visible in monitoring
- Load balancer may route traffic to unhealthy pods briefly

## Diagnosis Steps

### Step 1: Identify the Restarting Pod and Reason

```bash
# List pods with restart counts
kubectl get pods -n auction-system -l app=<service-name> -o wide --sort-by='.status.containerStatuses[0].restartCount'

# Get pod events for the restarting pod
kubectl describe pod <pod-name> -n auction-system

# Check pod status details
kubectl get pod <pod-name> -n auction-system -o jsonpath='{.status.containerStatuses[*].lastState}'
```

Look for:
- `OOMKilled` — Out of memory
- `Error` — Application crash
- `CrashLoopBackOff` — Repeated failures to start

### Step 2: Review Previous Container Logs

```bash
# Get logs from the crashed container (previous instance)
kubectl logs <pod-name> -n auction-system --previous --tail=200

# Check for panic/fatal logs
kubectl logs <pod-name> -n auction-system --previous | grep -i "panic\|fatal\|error"

# Check init container logs if startup is failing
kubectl logs <pod-name> -n auction-system -c <init-container-name> --previous
```

### Step 3: Check Resource Limits

```bash
# Check current resource usage vs limits
kubectl top pod <pod-name> -n auction-system

# Check pod resource requests/limits
kubectl get pod <pod-name> -n auction-system -o jsonpath='{.spec.containers[*].resources}'

# Check if OOMKill events occurred
kubectl get events -n auction-system --field-selector reason=OOMKilling --sort-by='.lastTimestamp'
```

### Step 4: Verify External Dependencies at Startup

```bash
# Check if Vault/SealedSecrets is accessible
kubectl get pods -n vault -l app=vault

# Check if database is accessible
kubectl exec -n auction-system <running-pod> -- pg_isready -h <db-host> -p 5432

# Check if Redis is accessible  
kubectl exec -n auction-system <running-pod> -- redis-cli -h <redis-host> ping
```

If the pod fails during startup due to secret retrieval failure (Vault unavailable), it will retry 5 times with exponential backoff (max 60s) before failing.

### Step 5: Check Health Check Configuration

```bash
# Review liveness and readiness probe configuration
kubectl get pod <pod-name> -n auction-system -o jsonpath='{.spec.containers[*].livenessProbe}'
kubectl get pod <pod-name> -n auction-system -o jsonpath='{.spec.containers[*].readinessProbe}'
```

Verify:
- `startupProbe` has sufficient `failureThreshold × periodSeconds` for slow starts
- Liveness probe endpoint is responding correctly
- Database migration job completed before app startup

## Resolution

### Immediate Actions

1. **If OOMKilled:** Increase memory limits in the deployment:
   ```bash
   kubectl set resources deployment/<service-name> -n auction-system --limits=memory=<new-limit>
   ```

2. **If application crash (panic/fatal):** Check logs for the root cause. If caused by a recent deployment, roll back:
   ```bash
   kubectl rollout undo deployment/<service-name> -n auction-system
   ```

3. **If Vault/secrets unavailable:** Check Vault pod health and restore access:
   ```bash
   kubectl get pods -n vault
   kubectl logs -n vault <vault-pod>
   ```

4. **If database migration failing:** Check migration job status and fix:
   ```bash
   kubectl get jobs -n auction-system -l app=<service-name>-migration
   kubectl logs job/<migration-job-name> -n auction-system
   ```

5. **If health check misconfigured:** Adjust probe parameters (increase timeout/period/failure threshold):
   ```bash
   kubectl edit deployment/<service-name> -n auction-system
   ```

### Post-Incident

- Verify pod stabilizes without further restarts
- Confirm PDB is maintaining minimum availability
- Review resource limits and adjust if under-provisioned
- Update startup probe if initialization time has grown
- Document root cause in incident report

## Escalation

| Level | Contact | When |
|-------|---------|------|
| L1 | On-call SRE | Alert fires |
| L2 | Service team lead | Pod not stabilizing after 15 minutes |
| L3 | Platform engineering | Infrastructure-related restart (node issues) |
| L4 | Engineering manager | Service unavailable > 30 minutes |

**PagerDuty Service:** `auction-system-warning`  
**Slack Channel:** `#auction-incidents`

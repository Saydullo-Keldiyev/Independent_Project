# Runbook: High Error Rate

## Alert Description

**Alert Name:** HighErrorRate  
**Severity:** Critical  
**Trigger Condition:** Service error rate exceeds 5% over a 5-minute window  
**Prometheus Expression:** `rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05`

## Symptoms

- Users reporting HTTP 500 or other 5xx errors
- Increased error count visible in Grafana service-level dashboard
- Possible service degradation or complete unavailability
- Elevated circuit breaker state transitions (closed → open)
- Downstream services may also report failures due to cascading effects

## Diagnosis Steps

### Step 1: Identify the Affected Service

```bash
# Check which service is producing errors
kubectl get pods -n auction-system -l app=<service-name> --sort-by='.status.startTime'

# View recent pod events
kubectl describe pod <pod-name> -n auction-system
```

Check Grafana service-level dashboard for the specific service showing elevated error rates.

### Step 2: Review Application Logs

```bash
# Stream logs from the affected service
kubectl logs -n auction-system -l app=<service-name> --tail=200 -f

# Filter for error-level logs
kubectl logs -n auction-system -l app=<service-name> --tail=500 | grep '"level":"error"'
```

Look for:
- Repeated error patterns (connection refused, timeout, panic)
- Correlation IDs associated with failing requests
- Stack traces or panic logs

### Step 3: Check Dependency Health

```bash
# Check PostgreSQL connectivity
kubectl exec -n auction-system <pod-name> -- pg_isready -h <db-host> -p 5432

# Check Redis connectivity
kubectl exec -n auction-system <pod-name> -- redis-cli -h <redis-host> ping

# Check Kafka broker status
kubectl get pods -n kafka -l app=kafka
```

Verify circuit breaker metrics — if breakers are open, the root cause may be a downstream dependency.

### Step 4: Check Resource Utilization

```bash
# Check pod resource usage
kubectl top pods -n auction-system -l app=<service-name>

# Check for OOMKilled events
kubectl get events -n auction-system --field-selector reason=OOMKilled
```

Review HPA status to verify if autoscaling is responding appropriately.

### Step 5: Review Recent Deployments

```bash
# Check recent deployment history
kubectl rollout history deployment/<service-name> -n auction-system

# Check ArgoCD application sync status
argocd app get auction-system-<service-name>
```

If a recent deployment correlates with the error spike, consider a rollback.

## Resolution

### Immediate Actions

1. **If caused by a recent deployment:** Roll back to the previous known-good version:
   ```bash
   kubectl rollout undo deployment/<service-name> -n auction-system
   ```

2. **If caused by resource exhaustion:** Scale up the service:
   ```bash
   kubectl scale deployment/<service-name> -n auction-system --replicas=<N>
   ```

3. **If caused by dependency failure:** Check if the dependency is recovering. Circuit breakers should prevent cascading failures. If the dependency is permanently down, engage the relevant team.

4. **If caused by configuration error:** Correct the configuration in the Vault/SealedSecrets and restart pods:
   ```bash
   kubectl rollout restart deployment/<service-name> -n auction-system
   ```

### Post-Incident

- Verify error rate returns below 5% threshold
- Confirm alert resolves automatically
- Document root cause in incident report
- Create follow-up tickets for permanent fixes

## Escalation

| Level | Contact | When |
|-------|---------|------|
| L1 | On-call SRE | Alert fires |
| L2 | Service team lead | Not resolved within 15 minutes |
| L3 | Engineering manager | Customer-facing impact > 30 minutes |
| L4 | VP Engineering | Complete service outage > 1 hour |

**PagerDuty Service:** `auction-system-critical`  
**Slack Channel:** `#auction-incidents`

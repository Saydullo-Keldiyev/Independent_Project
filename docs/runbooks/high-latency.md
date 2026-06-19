# Runbook: High Latency

## Alert Description

**Alert Name:** HighLatency  
**Severity:** Warning  
**Trigger Condition:** Service P99 latency exceeds 2 seconds over a 5-minute window  
**Prometheus Expression:** `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 2`

## Symptoms

- Users reporting slow page loads or API response times
- Elevated P99/P95 latency visible in Grafana service-level dashboard
- WebSocket connections may experience delayed bid updates
- Increased timeout errors in upstream services
- Circuit breakers may begin transitioning to half-open/open states

## Diagnosis Steps

### Step 1: Identify the Latency Source

```bash
# Check latency by endpoint in Grafana
# Query: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{service="<service>"}[5m])) by (handler)
```

Determine if the latency is:
- Isolated to specific endpoints
- Affecting all endpoints uniformly
- Correlating with specific downstream services

### Step 2: Check Database Performance

```bash
# Check active PostgreSQL connections and long-running queries
kubectl exec -n auction-system <db-pod> -- psql -c "SELECT pid, now() - pg_stat_activity.query_start AS duration, query FROM pg_stat_activity WHERE state != 'idle' ORDER BY duration DESC LIMIT 10;"

# Check connection pool utilization metrics
# Prometheus: db_pool_active_connections / db_pool_max_connections
```

Look for:
- Long-running queries (>1s)
- Lock contention (SELECT FOR UPDATE waits)
- Connection pool exhaustion (active == max)

### Step 3: Check Redis and Kafka Performance

```bash
# Check Redis latency
kubectl exec -n auction-system <redis-pod> -- redis-cli --latency

# Check Redis slow log
kubectl exec -n auction-system <redis-pod> -- redis-cli slowlog get 10

# Check Kafka consumer lag
# Prometheus: kafka_consumer_group_lag{consumer_group="<group>"}
```

### Step 4: Analyze Resource Saturation

```bash
# Check CPU throttling
kubectl top pods -n auction-system -l app=<service-name>

# Check if pods are CPU-throttled
kubectl get pods -n auction-system -l app=<service-name> -o jsonpath='{.items[*].spec.containers[*].resources}'
```

Review Grafana infrastructure dashboard for:
- CPU utilization >80%
- Memory pressure (>85% triggers request rejection)
- Network I/O saturation

### Step 5: Review Concurrent Load

```bash
# Check current request rate
# Prometheus: rate(http_requests_total{service="<service>"}[1m])

# Check WebSocket connection count (Bid Service)
# Prometheus: websocket_connections_active{service="bid-service"}
```

Determine if load spike correlates with latency increase. Check if HPA is scaling appropriately.

## Resolution

### Immediate Actions

1. **If caused by database slow queries:** Kill long-running queries and investigate missing indexes:
   ```bash
   kubectl exec -n auction-system <db-pod> -- psql -c "SELECT pg_cancel_backend(<pid>);"
   ```

2. **If caused by resource saturation:** Manually scale up:
   ```bash
   kubectl scale deployment/<service-name> -n auction-system --replicas=<N+2>
   ```

3. **If caused by connection pool exhaustion:** Restart pods to reset connections:
   ```bash
   kubectl rollout restart deployment/<service-name> -n auction-system
   ```

4. **If caused by Redis slow operations:** Check for large key scans and optimize queries.

5. **If caused by traffic spike:** Verify rate limiting is active. Consider temporarily lowering rate limits for non-critical endpoints.

### Post-Incident

- Verify P99 latency returns below 2s threshold
- Review and optimize slow queries
- Adjust connection pool sizes if needed
- Consider adding caching for frequently accessed data
- Update HPA thresholds if scaling was too slow

## Escalation

| Level | Contact | When |
|-------|---------|------|
| L1 | On-call SRE | Alert fires |
| L2 | Service team lead | Not resolved within 20 minutes |
| L3 | DBA team | Database-related latency > 30 minutes |
| L4 | Engineering manager | User-facing impact > 45 minutes |

**PagerDuty Service:** `auction-system-warning`  
**Slack Channel:** `#auction-incidents`

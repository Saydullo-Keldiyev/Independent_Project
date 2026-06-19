# Runbook: Kafka Consumer Lag

## Alert Description

**Alert Name:** KafkaConsumerLag  
**Severity:** Critical  
**Trigger Condition:** Consumer lag exceeds 10,000 messages for 5 minutes  
**Prometheus Expression:** `kafka_consumer_group_lag > 10000`

## Symptoms

- Event processing delays (notifications, settlements, bid updates)
- Users not receiving real-time notifications
- Auction settlement delays
- Growing DLQ (Dead Letter Queue) size
- Stale data in search indexes
- WebSocket clients receiving outdated bid information

## Diagnosis Steps

### Step 1: Identify the Affected Consumer Group

```bash
# Check consumer group lag per topic
# Prometheus: kafka_consumer_group_lag{consumer_group=~".*"} > 10000

# List consumer groups and their status
kubectl exec -n kafka <kafka-pod> -- kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list

# Describe the lagging consumer group
kubectl exec -n kafka <kafka-pod> -- kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group <consumer-group>
```

### Step 2: Check Consumer Pod Health

```bash
# Check if consumer pods are running
kubectl get pods -n auction-system -l app=<consumer-service>

# Check for recent restarts
kubectl get pods -n auction-system -l app=<consumer-service> -o wide --sort-by='.status.containerStatuses[0].restartCount'

# Check consumer logs for errors
kubectl logs -n auction-system -l app=<consumer-service> --tail=200 | grep -i "error\|failed\|timeout"
```

### Step 3: Check Message Processing Performance

```bash
# Check processing duration histogram
# Prometheus: histogram_quantile(0.99, rate(kafka_message_processing_duration_seconds_bucket[5m]))

# Check retry count
# Prometheus: rate(kafka_message_retry_total[5m])

# Check DLQ size growth
# Prometheus: kafka_dlq_messages_total
```

Look for:
- Processing time spikes (indicating slow downstream calls)
- High retry rates (indicating repeated failures)
- Growing DLQ (indicating messages cannot be processed)

### Step 4: Check Kafka Broker Health

```bash
# Check broker pod status
kubectl get pods -n kafka -l app=kafka

# Check topic partition distribution
kubectl exec -n kafka <kafka-pod> -- kafka-topics.sh --bootstrap-server localhost:9092 --describe --topic <topic>

# Check under-replicated partitions
kubectl exec -n kafka <kafka-pod> -- kafka-topics.sh --bootstrap-server localhost:9092 --describe --under-replicated-partitions
```

### Step 5: Check Dependencies Used During Processing

```bash
# Verify Redis availability (idempotency store)
kubectl exec -n auction-system <consumer-pod> -- redis-cli -h <redis-host> ping

# Verify PostgreSQL availability
kubectl exec -n auction-system <consumer-pod> -- pg_isready -h <db-host> -p 5432

# Check circuit breaker states for downstream services
# Prometheus: circuit_breaker_state{service="<downstream>"}
```

## Resolution

### Immediate Actions

1. **If consumers are crashed/restarting:** Investigate crash reason and fix:
   ```bash
   kubectl describe pod <consumer-pod> -n auction-system
   kubectl logs <consumer-pod> -n auction-system --previous
   ```

2. **If processing is slow due to downstream dependency:** Fix the dependency or temporarily increase consumer instances:
   ```bash
   kubectl scale deployment/<consumer-service> -n auction-system --replicas=<N+2>
   ```

3. **If topic has too few partitions for consumer parallelism:** Add partitions (non-reversible):
   ```bash
   kubectl exec -n kafka <kafka-pod> -- kafka-topics.sh --bootstrap-server localhost:9092 --alter --topic <topic> --partitions <N>
   ```

4. **If messages are poisonous (always failing):** Check DLQ and consider skipping:
   ```bash
   # Review DLQ messages
   kubectl exec -n kafka <kafka-pod> -- kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic <topic>.dlq --from-beginning --max-messages 5
   ```

5. **If idempotency store (Redis) is down:** Consumers should continue processing with degraded idempotency (per design). Restore Redis ASAP to prevent duplicate processing.

### Post-Incident

- Verify consumer lag returns below 10,000 messages
- Review DLQ for messages requiring manual reprocessing
- Check for duplicate event processing if Redis was unavailable
- Consider adjusting consumer concurrency or partition count
- Update HPA config for consumer deployments if needed

## Escalation

| Level | Contact | When |
|-------|---------|------|
| L1 | On-call SRE | Alert fires |
| L2 | Service team lead | Lag not decreasing after 10 minutes |
| L3 | Kafka platform team | Broker issues or partition problems |
| L4 | Engineering manager | Business impact (settlements delayed > 30 min) |

**PagerDuty Service:** `auction-system-critical`  
**Slack Channel:** `#auction-incidents`

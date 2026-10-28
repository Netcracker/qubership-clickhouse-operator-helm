---
name: troubleshoot-clickhouse
description: Diagnose and resolve query processing errors, ZooKeeper/replication failures, memory limit/OOM issues, kernel compatibility problems, merge/part backlog, and user/profile configuration issues, query performance, server settings, users/profiles, replication/ZooKeeper coupling, or documented ClickHouse failure modes. Match the reported symptom to the documented troubleshooting section; fall back to a general diagnostic checklist when no specific match exists.
---

## ClickHouse Operator Troubleshooting Skill — Summary

Comprehensive troubleshooting guide for **Qubership ClickHouse Operator**, a Kubernetes operator managing ClickHouse clusters via Helm and the ClickHouse operator framework.

## Core Architecture

The operator manages:
- **ClickhouseInstallation (CRD)** — Cluster provisioning, scaling, replication
- **Service components** — site-manager, backup-orchestrator, dbaas-adapter, sidecar, secretmonitor, hook
- **Replication layer** — ZooKeeper-backed replicated tables with sharding
- **High availability** — Active/standby pairs with switchover capability via site-manager

## Six Common Failure Categories

1. **Query Processing Errors** — Broken connections, timeout errors, query execution failures
2. **Replication & ZooKeeper Issues** — Connection loss, insert failures, replication queue buildup
3. **Memory & Resource Limits** — OOMKilled pods, memory limit exceeded errors, CPU throttling
4. **Merge & Part Management** — Too many parts, merge backlog, insert latency spikes
5. **Kernel & Compatibility Issues** — RENAME EXCHANGE not supported, filesystem problems
6. **Configuration & User Management** — Profile misconfigurations, user/network access issues

## Quick Diagnostic Workflow

Start with logs from affected component:
- Operator: `kubectl logs deployment/clickhouse`
- ClickHouse pod: `kubectl logs <pod-name> -c clickhouse-pod`
- clichouse Backup : `kubectl logs <pod-name> -c clickhouse-backup`

Then check resource state via `kubectl describe ClickhouseInstallation` and recent events. Use the symptom-to-section mapping table below to locate specific guidance.

## How to use the reference file

1. Grep issue headers:
   `grep -n "^## " references/troubleshooting.md`
2. If no results:
   `grep -n "^##[^# ]" references/troubleshooting.md`
3. Read only that section: offset at its line number, limit through to the next header's line number. Never load the whole file for one lookup.

## Symptom → reference section

| Symptom | Section in references/troubleshooting.md |
|---|---|
| Query processing error / broken connection | Query Processing |
| ZooKeeper connection loss after DR switchover | ClickHouse lost connection to Zookeeper after switchover |
| Too many parts / insert latency spikes / merge backlog | Merge backlog / part count climbing |
| Replicated table insert failures / ZooKeeper unavailability | ZooKeeper coupling failures |
| Memory limit for query exceeded / OOMKilled | Memory limit (for query) exceeded |
| RENAME EXCHANGE not supported / kernel issue | RENAME EXCHANGE is not supported |

## Configuration

ClickHouse configuration is managed through Helm values and the ClickHouse operator. Do not modify generated ClickHouse configuration files directly inside the container.

### Server settings (config.xml equivalent)

Use:

`clickhouseCluster.configuration.settings.<parameter>`

Examples:

```yaml
clickhouseCluster:
  configuration:
    settings:
      "max_connections": 4096
      "mark_cache_size": 5368709120
      "logger/log": "/var/log/clickhouse-server/clickhouse-server.log"
      "logger/level": "information"
```

### Profiles (profiles block in users.xml)

Use:

`clickhouseCluster.configuration.profiles.<profile>/<parameter>`

Examples:

```yaml
clickhouseCluster:
  configuration:
    profiles:
      "default/max_memory_usage": 10000000000
      "readonly/readonly": 1
```

### Users (users block in users.xml)

ClickHouse users are configured through:

`clickhouseCluster.users.<username>`

Example:

```yaml
clickhouseCluster:
  users:
    analyst:
      profile: readonly
      password: "<sha256-hash>"
      networks:
        - "10.0.0.0/8"
```

User configuration may include:

- `profile`
- `password`
- `networks`

Do not expose or invent real passwords or credentials, the operator expects hashed values.

## Config value conventions

- Helm values are the source of truth.
- Do not recommend direct edits to generated ClickHouse XML configuration.
- ClickHouse configuration profiles are addressed as `configuration.profiles.<profile>/<param>`.
- Prometheus alert thresholds are configured through `clickhouseCluster.prometheusRules.<paramName>`.
- TLS certificates are read from `/tls/ca.crt` at runtime.

## Guardrails

Do not recommend:

- Direct XML config edits inside the container (`/etc/clickhouse-server/config.d/`, `/etc/clickhouse-server/users.d/`) — these are overwritten on the next operator reconcile.
- `clickhouse-client` `SYSTEM RELOAD CONFIG` for persistent settings — the operator regenerates the configuration.
- `ALTER TABLE … MODIFY SETTING` against operator-managed tables for settings the operator owns — changes are reverted by the operator.
- `kubectl exec … clickhouse-client` for any DDL/DML on production tables — destructive and bypasses the operator reconciliation loop.
- `SYSTEM RESTART REPLICA` without first confirming ZooKeeper connectivity and replication path consistency.
- Manual deletion of ZooKeeper znodes for table replication paths.
- Scaling replicas down to zero — at least one replica must remain available.
- Clearing replicated data or performing destructive recovery steps before ensuring that at least one replica per shard remains available.

## Cluster Conventions

- **CRD**: `ClickhouseInstallation` (group: `clickhouse.altinity.com`)
- **Healthy CR status**: `.status.status` = `"Completed"`
- **Supported versions**: 243 (24.3.18) and 258 (25.8.18)
- **Service components**: site-manager, backup-orchestrator, dbaas-adapter, sidecar, secretmonitor, hook
- **DR topology**: Active/standby pairs with switchover via site-manager
- **Replica availability**: At least one replica per shard must remain available before destructive recovery

## Common Failure Modes

### 1. ClickHouse Pod Failures

**Pods Not Starting (CrashLoopBackOff)**

Symptom: ClickHouse pods in `CrashLoopBackOff` or `Error` state

Quick Checks:
```bash
# Pod events
kubectl describe pod <pod-name> -n <namespace>

# Container logs
kubectl logs <pod-name> -c clickhouse-pod -n <namespace>

```

Common Issues:
- PVC mount failures (storage class misconfiguration)
- Insufficient resources (CPU/memory requests exceed limits)
- ClickHouse config parsing errors
- Network access to ZooKeeper blocked
- TLS certificate issues

Resolution:
1. Check PVC status: `kubectl get pvc -n <namespace>`
2. Verify storage class exists: `kubectl get storageclass`
3. Review pod resource limits vs. requests: `kubectl describe pod <pod-name>`


**Pod Restart Loops**

Symptom: ClickHouse pod restarts repeatedly, stays alive briefly

Causes:
- Memory limit too low for query workload
- Invalid query profile configuration
- ZooKeeper quorum not reached
- Incompatible ClickHouse version

Resolution:
```bash
# Check restart count
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[0].restartCount}'

# Increase memory limit
# Edit values.yaml: resources.limits.memory: 16Gi
helm upgrade <release> -f values.yaml

# Monitor pod stability
kubectl logs <pod-name> -c clickhouse-pod --tail=200 -n <namespace>
```

### 2. ZooKeeper & Replication Failures

**ZooKeeper Connection Loss**

Symptom: Errors like `Cannot read from ZooKeeper: Connection timed out` or `ZooKeeper session expired`

Diagnostic Steps:
```bash
# Check ZooKeeper pod health
kubectl get pods -n <zk-namespace> | grep zookeeper

# Verify ClickHouse can reach ZooKeeper
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT * FROM system.zookeeper WHERE path='/clickhouse'"

# Check ZooKeeper logs
kubectl logs <zk-pod> -n <zk-namespace> | grep -i "connection\|timeout\|error"
```

Common Causes:
- ZooKeeper pods not running or unhealthy
- Network policy blocking ClickHouse ↔ ZooKeeper traffic
- ZooKeeper endpoints misconfigured in Helm values
- ZooKeeper quorum lost (less than majority running)

Resolution:
1. Ensure ZooKeeper is running: `kubectl get statefulset -n <zk-namespace>`
2. Verify connectivity: `kubectl exec <clickhouse-pod> -- nc -zv <zk-host>:2181`
3. Check Helm values: `zookeeper.servers[].host` and `zookeeper.servers[].port`
4. Review network policies: `kubectl get networkpolicy -A`

**Replication Queue Buildup**

Symptom: Insert latency increases, replica queues grow, `INSERT` queries hang

Quick Check:
```bash
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT * FROM system.replication_queue LIMIT 10"
```

Causes:
- Replicas lagging behind leader
- ZooKeeper latency or unavailability
- Replica stuck in initialization
- Network latency between replicas

Resolution:
1. Check replica status: `SELECT * FROM system.replicas`
2. Monitor queue depth: `SELECT * FROM system.replication_queue`
3. If replica is stuck: `SYSTEM RESTART REPLICA <table>` (after confirming ZK connectivity)
4. Increase replication thread count if safe: `max_replication_tasks_for_replicated_tables`

**Table Insert Failures**

Symptom: `Cannot write to replicated table: ClickHouse cannot reach ZooKeeper` or insert hangs

Diagnostic:
```bash
# Check if table exists on all replicas
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SHOW TABLES" -d <database>

# Check replication path in ZooKeeper
kubectl exec <zk-pod> -n <zk-namespace> -- \
  /opt/zookeeper/bin/zkCli.sh -server localhost:2181 \
  get /clickhouse/tables/<cluster>/<database>/<table>/metadata
```

Resolution:
1. Verify table definition is consistent across all replicas
2. Check ZooKeeper replication path exists
3. Ensure at least N-1 replicas are running (N = replica count)
4. Use `SYSTEM SYNC REPLICA` to force replica sync

### 3. Memory & Resource Limits

**Memory Limit Exceeded (OOMKilled)**

Symptom: Pod terminated with `OOMKilled` or ClickHouse logs show `Memory limit (for query) exceeded`

Quick Check:
```bash
# Verify pod was OOMKilled
kubectl describe pod <pod-name> | grep -i "oom\|memory"

# Check query memory usage
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT query, memory_usage FROM system.processes"

# Check configured memory limit
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT value FROM system.settings WHERE name='max_memory_usage'"
```

Common Causes:
- Query memory limit too low for result set
- Large JOIN operations exceed profile limit
- Aggregation on high cardinality column
- Memory leak in ClickHouse version

Resolution:
1. Increase pod memory limit in values.yaml: `resources.limits.memory`
2. Increase query profile limit: `clickhouseCluster.configuration.profiles.default/max_memory_usage`
3. For specific queries, use `SET max_memory_usage = <value>`
4. Check for memory leaks in ClickHouse version (upgrade if available)

**Resource Request/Limit Mismatch**

Symptom: Node fails to schedule ClickHouse pods (insufficient resources)

Fix:
```yaml
clickhouseCluster:
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "16Gi"
```

### 4. Merge & Part Management

**Too Many Parts**

Symptom: Insert latency spikes, ClickHouse logs show `Too many parts`

Quick Check:
```bash
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT table, partition, count(*) as parts FROM system.parts GROUP BY table, partition"
```

Common Causes:
- Merge threads insufficient for write load
- Parts too small (merge overhead > benefit)
- Merge policy not aggressive enough

Resolution:
1. Force merge: `OPTIMIZE TABLE <table> FINAL`
2. Increase merge threads: `background_pool_size` (default: 16)
3. Tune merge policy: `parts_to_throw_insert`, `parts_to_delay_insert`
4. Monitor with: `SELECT * FROM system.merges`

**Merge Backlog**

Symptom: Pending merges accumulate, queries slow down

Diagnostic:
```bash
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT * FROM system.merges"
```

Resolution:
1. Increase `background_pool_size` for more concurrent merges
2. Reduce part size target (allows faster merges)
3. Trigger manual merge: `OPTIMIZE TABLE <table>`
4. Check disk I/O: `iostat -x 1 5`

### 5. Query Performance Issues

**Slow Query Execution**

Symptom: Queries timeout or return results slowly

Quick Check:
```bash
# Enable query log
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT query, query_duration_ms, read_rows FROM system.query_log ORDER BY event_time DESC LIMIT 10"

# Check active queries
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT query, elapsed FROM system.processes"
```

Common Causes:
- Missing indices on filter columns
- Insufficient memory for aggregation
- Full table scan on large dataset
- Network latency for distributed queries

Resolution:
1. Check table schema: `DESC <table>`
2. Use `EXPLAIN` to see query plan: `EXPLAIN <query>`
3. Add primary key indices if beneficial
4. Increase `max_execution_time` if queries are legitimately slow

### 6. Configuration & User Issues

**User Authentication Failure**

Symptom: `Authentication failed: wrong password` or `User <name> is not allowed to connect from <host>`

Quick Check:
```bash
# List configured users
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -u default -q "SELECT * FROM system.users"

# Check network access for user
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  clickhouse-client -q "SELECT name, host_ip, host_names, host_names_regexp, host_names_like
    FROM system.users"
```

Resolution:
1. Verify user exists and password matches
2. Check network access: users.yaml `networks` section
3. Verify user profile is active
4. For default user, check password hash matches config

**Profile Configuration Error**

Symptom: Query fails with `Setting <name> is not available for profile <profile>`

Fix:
```yaml
clickhouseCluster:
  configuration:
    profiles:
      "custom/max_memory_usage": 10000000000
      "custom/readonly": 0
```

### 7. Kernel & Filesystem Issues

**RENAME EXCHANGE Not Supported**

Symptom: `RENAME EXCHANGE is not supported` error during merge operations

Cause: Filesystem doesn't support `renameat2()` syscall (common on certain ext4 versions or virtualized storage)

Resolution:
1. Check kernel version: `uname -r`
2. Upgrade to kernel 4.16+ if available
3. Use `fsync_part_directory: 0` as workaround (risky, not recommended)
4. Consider upgrading filesystem to ext4 with newer kernel

**Disk Space Issues**

Symptom: ClickHouse pod crashes or becomes read-only

Quick Check:
```bash
kubectl exec <clickhouse-pod> -c clickhouse-pod -- \
  df -h /var/lib/clickhouse
```

Resolution:
1. Expand PVC: `kubectl patch pvc <pvc-name> -p '{"spec":{"resources":{"requests":{"storage":"100Gi"}}}}'`
2. Delete old data: `ALTER TABLE <table> DELETE WHERE <condition>`
3. Detach partitions: `ALTER TABLE <table> DETACH PARTITION <partition>`

## Diagnostic Workflow

### Initial Triage

Identify the failure component:

```bash
# Operator logs
kubectl logs deployment/clickhouse-operator -n <namespace>

# ClickHouse pod logs
kubectl logs <pod-name> -c clickhouse-pod -n <namespace>

# Check resource state
kubectl describe ClickhouseInstallation <name> -n <namespace>
kubectl get pods -n <namespace> -o wide

# Recent events
kubectl get events -n <namespace> --sort-by='.lastTimestamp' | tail -20
```

### Narrowing Down Issues

| Symptom | Section |
|---|---|
| CrashLoopBackOff, pod won't start | ClickHouse Pod Failures |
| Cannot reach ZooKeeper | ZooKeeper & Replication Failures |
| Insert latency high / INSERT hangs | Replication Queue Buildup or Too Many Parts |
| Memory limit exceeded / OOMKilled | Memory & Resource Limits |
| Query slow / timeout | Query Performance Issues |
| User authentication fails | Configuration & User Issues |
| RENAME EXCHANGE not supported | Kernel & Filesystem Issues |

### Using the Full Troubleshooting Guide

For deeper issues not covered above, load only the relevant section from `references/troubleshooting.md` — do NOT read the full file.

Steps:
1. Grep section headers to find line numbers: `grep -n "^## " references/troubleshooting.md`
2. Match symptom to section from the table above
3. Read only that section using offset and limit

Example: If grep shows line 42 has "Memory issues", read from line 42 to the next header.

## Key Helm Chart Parameters

### Critical Configuration

**ClickHouse Cluster Definition**:
```yaml
clickhouseCluster:
  layout:
    shardsCount: 1
    replicasCount: 3  # At least 1 replica per shard
  image: "clickhouse/clickhouse-server:latest"
```

**ZooKeeper Configuration** (for replication):
```yaml
zookeeper:
  servers:
    - name: "zk-0"
      host: "zk-0.zk-headless.default.svc.cluster.local"
      port: 2181
  sessionTimeoutMs: 30000
```

**Server Settings** (config.xml):
```yaml
clickhouseCluster:
  configuration:
    settings:
      "max_connections": 4096
      "max_concurrent_queries": 100
      "max_replication_tasks_for_replicated_tables": 192
      "background_pool_size": 16
      "background_schedule_pool_size": 128
      "logger/level": "information"
      "logger/log": "/var/log/clickhouse-server/clickhouse-server.log"
```

**Query Profiles**:
```yaml
clickhouseCluster:
  configuration:
    profiles:
      "default/max_memory_usage": 10000000000
      "default/max_execution_time": 600
      "readonly/readonly": 1
      "readonly/allow_ddl": 0
```

**User Configuration**:
```yaml
clickhouseCluster:
  users:
    default:
      profile: "default"
      password: "<sha256-hash>"
      networks:
        - "127.0.0.1"
        - "::1"
      quotas: "default"
    analyst:
      profile: "readonly"
      password: "<sha256-hash>"
      networks:
        - "10.0.0.0/8"
```

**Pod Resources**:
```yaml
clickhouseCluster:
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "16Gi"
  volumeClaimTemplate:
    accessModes: ["ReadWriteOnce"]
    resources:
      requests:
        storage: "100Gi"
    storageClassName: "fast-ssd"
```

## Log Analysis Tips

### ClickHouse Server Logs

Key patterns to grep:

```bash
# Replication errors
kubectl logs <pod-name> -c clickhouse-pod | grep -iE "replication|replica"

# ZooKeeper connectivity
kubectl logs <pod-name> -c clickhouse-pod | grep -iE "zookeeper|keeper"

# Merge operations
kubectl logs <pod-name> -c clickhouse-pod | grep -iE "merge|merging"

# Query execution
kubectl logs <pod-name> -c clickhouse-pod | grep -iE "query|execution"

# Memory issues
kubectl logs <pod-name> -c clickhouse-pod | grep -i "memory"

# Startup/shutdown
kubectl logs <pod-name> -c clickhouse-pod | grep -iE "starting|shutdown|ready"
```

### Operator Logs

```bash
# CRD reconciliation
kubectl logs deployment/clickhouse-operator | grep -iE "ClickhouseInstallation|reconcil"

# Configuration changes
kubectl logs deployment/clickhouse-operator | grep -i "config"

# Resource creation
kubectl logs deployment/clickhouse-operator | grep -iE "create|update|delete"
```

## Integration Points

### ZooKeeper Health

Check ZooKeeper cluster status:
```bash
kubectl exec <zk-pod> -n <zk-namespace> -- \
  /opt/zookeeper/bin/zkServer.sh status

# Check ClickHouse replication paths
kubectl exec <zk-pod> -n <zk-namespace> -- \
  /opt/zookeeper/bin/zkCli.sh -server localhost:2181 \
  ls /clickhouse/tables
```

### Monitoring & Alerting

Check metrics:
```bash
# Port-forward to ClickHouse metrics endpoint
kubectl port-forward <pod-name> 8123:8123

# Query system tables
curl "http://localhost:8123/?query=SELECT%20*%20FROM%20system.metrics"
```

## Prevention Best Practices

1. **Replication**: Verify ZooKeeper connectivity before deploying replicated tables
2. **Memory**: Monitor `system.processes` to identify memory-heavy queries before OOM
3. **Merges**: Configure `parts_to_throw_insert` to prevent unbounded part growth
4. **Upgrades**: Test version upgrades in non-production before production deployment
5. **Backups**: Regularly test backup/restore procedures
6. **Monitoring**: Set up alerts for replica lag, merge queue depth, part count
7. **Configuration**: Use dry-run validation before applying CRD changes: `kubectl apply --dry-run=server -f clickhouse.yaml`
8. **Network**: Ensure network policies allow ClickHouse ↔ ZooKeeper traffic
9. **Storage**: Use fast SSD storage classes; monitor I/O saturation
10. **Cluster topology**: Never scale replicas down to zero; maintain at least one replica per shard

## When to Escalate

Escalate to component maintainers when:

- ClickHouse crashes with segmentation fault (attach core dump)
- Data corruption suspected (checksum failures, query results inconsistent across replicas)
- ZooKeeper quorum lost and recovery is unclear
- Replication stuck in unrecoverable state
- Upgrade rollback unsuccessful
- Performance unexplained by query patterns or cluster load

Include in escalation:
- Full operator logs: `kubectl logs deployment/clickhouse-operator`
- ClickHouse pod logs from affected replicas
- CRD YAML: `kubectl get ClickhouseInstallation -o yaml`
- ZooKeeper state dump (replication paths)
- `system.replicas` and `system.replication_queue` output
- Metrics from `system.metrics` and `system.events`


---
name: troubleshoot-clickhouse
description: Diagnose and resolve query processing errors, ZooKeeper/replication failures, memory limit/OOM issues, kernel compatibility problems, merge/part backlog, and user/profile configuration issues, query performance, server settings, users/profiles, replication/ZooKeeper coupling, or documented ClickHouse failure modes. Match the reported symptom to the documented troubleshooting section; fall back to a general diagnostic checklist when no specific match exists.

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

## Cluster conventions

- CRD: `ClickhouseInstallation` (group: `clickhouse.altinity.com`)
- Healthy CR status: `.status.status` = `"Completed"`
- Two ClickHouse versions are maintained: 243 (24.3) and 258 (25.8).
- Service components:
  - site-manager
  - backup-orchestrator
  - dbaas-adapter
  - sidecar
  - secretmonitor
  - hook
- DR uses active/standby pairs with switchover capability.
- At least one replica per shard must remain available before attempting any data-clearing recovery step.

## General diagnostic checklist

When no specific troubleshooting section matches:

1. Check the `ClickhouseInstallation` status.
2. Check ClickHouse pod readiness and restart/OOM status.
3. Check ClickHouse server logs for the reported error.
4. Check operator reconciliation events and logs.
5. Check ClickHouse replica status and replication queues.
6. Check ZooKeeper connectivity when replication is involved.
7. Check disk capacity and filesystem pressure.
8. Check memory and CPU utilization against configured limits.
9. Check the effective ClickHouse profile and setting involved in the failure.
10. Check whether the issue affects one replica, one shard, or the entire cluster.
11. Avoid destructive recovery actions until the replication state and data availability are understood.
12. Apply configuration fixes through Helm values and redeploy/reconcile the operator-managed resources.
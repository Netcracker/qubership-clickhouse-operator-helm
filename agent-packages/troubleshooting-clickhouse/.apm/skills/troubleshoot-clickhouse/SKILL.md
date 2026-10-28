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

## Quick Diagnostic Workflow

Start with logs from affected component:
- Operator: `kubectl logs deployment/clickhouse`
- ClickHouse pod: `kubectl logs <pod-name> -c clickhouse-pod`
- Backup sidecar: `kubectl logs <pod-name> -c backup-sidecar`

Then check resource state via `kubectl describe ClickhouseInstallation` and recent events. Use the symptom-to-section mapping table below to locate specific guidance.

## Symptom → Reference Section Mapping

| Symptom | Section in references/troubleshooting.md |
|---|---|
| Query processing error / broken connection | Query Processing |
| ZooKeeper connection loss after DR switchover | ClickHouse lost connection to Zookeeper after switchover |
| Too many parts / insert latency spikes / merge backlog | Merge backlog / part count climbing |
| Replicated table insert failures / ZooKeeper unavailability | ZooKeeper coupling failures |
| Memory limit for query exceeded / OOMKilled | Memory limit (for query) exceeded |
| RENAME EXCHANGE not supported / kernel issue | RENAME EXCHANGE is not supported |

## How to Use the Reference File

1. Grep issue headers:
   `grep -n "^## " references/troubleshooting.md`
2. If no results:
   `grep -n "^##[^# ]" references/troubleshooting.md`
3. Read only that section: offset at its line number, limit through to the next header's line number. Never load the whole file for one lookup.

## Configuration

ClickHouse configuration is managed through Helm values and the ClickHouse operator. Do not modify generated ClickHouse configuration files directly inside the container.

### Server Settings (config.xml equivalent)

Use: `clickhouseCluster.configuration.settings.<parameter>`

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

Use: `clickhouseCluster.configuration.profiles.<profile>/<parameter>`

Examples:
```yaml
clickhouseCluster:
  configuration:
    profiles:
      "default/max_memory_usage": 10000000000
      "readonly/readonly": 1
```

### Users (users block in users.xml)

Use: `clickhouseCluster.users.<username>`

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

User configuration may include: `profile`, `password`, `networks`

Do not expose or invent real passwords or credentials — the operator expects hashed values.

## Config Value Conventions

- Helm values are the source of truth.
- Do not recommend direct edits to generated ClickHouse XML configuration.
- ClickHouse configuration profiles are addressed as `configuration.profiles.<profile>/<param>`.
- Prometheus alert thresholds are configured through `clickhouseCluster.prometheusRules.<paramName>`.
- TLS certificates are read from `/tls/ca.crt` at runtime.

## Guardrails — Actions to Avoid

Do not recommend:

- Direct XML config edits inside the container (`/etc/clickhouse-server/config.d/`, `/etc/clickhouse-server/users.d/`) — overwritten on operator reconcile
- `clickhouse-client` `SYSTEM RELOAD CONFIG` for persistent settings — operator regenerates configuration
- `ALTER TABLE … MODIFY SETTING` for operator-managed table settings — reverted by operator
- `kubectl exec … clickhouse-client` for DDL/DML on production tables — destructive, bypasses reconciliation
- `SYSTEM RESTART REPLICA` without first confirming ZooKeeper connectivity
- Manual deletion of ZooKeeper znodes for table replication paths
- Scaling replicas down to zero — at least one replica must remain available
- Destructive recovery steps before ensuring at least one replica per shard remains

## Cluster Conventions

- **CRD**: `ClickhouseInstallation` (group: `clickhouse.altinity.com`)
- **Healthy CR status**: `.status.status` = `"Completed"`
- **Supported versions**: 243 (24.3.18) and 258 (25.8.18)
- **Service components**: site-manager, backup-orchestrator, dbaas-adapter, sidecar, secretmonitor, hook
- **DR topology**: Active/standby pairs with switchover via site-manager
- **Replica availability**: At least one replica per shard must remain available before destructive recovery

## Critical Helm Parameters

- `clickhouseCluster.configuration.settings.*` — Server settings (config.xml equivalent)
- `clickhouseCluster.configuration.profiles.<profile>/<parameter>` — Query profiles and per-profile settings
- `clickhouseCluster.users.<username>` — User configuration (profile, password, network access)
- `clickhouseCluster.prometheusRules.<paramName>` — Alert thresholds for monitoring
- ZooKeeper endpoints — Required for replicated table support
- Replica count per shard — At least one replica per shard must remain available

## Prevention Best Practices

1. Verify ZooKeeper connectivity before deploying replication tables
2. Monitor part count and merge queue health continuously
3. Test backup/restore procedures before production use
4. Monitor memory usage per query and adjust `max_memory_usage` limits proactively
5. Use dry-run validation before CRD changes: `kubectl apply --dry-run=server -f clickhouse.yaml`
6. Prevent part count explosion through aggressive merging configuration
7. Set up alerts for replica lag, merge queue depth, and part count
8. Ensure network policies allow ClickHouse ↔ ZooKeeper traffic
9. Use fast SSD storage classes; monitor I/O saturation
10. Never scale replicas to zero; maintain at least one replica per shard

## General Diagnostic Checklist

When no specific troubleshooting section matches:

1. Check the `ClickhouseInstallation` status via `kubectl describe ClickhouseInstallation/cluster`.
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

## When to Escalate

Escalate to component maintainers when:

- ClickHouse crashes with segmentation fault (attach core dump)
- Data corruption suspected (checksum failures, query results inconsistent across replicas)
- ZooKeeper quorum lost and recovery is unclear
- Replication stuck in unrecoverable state
- Upgrade rollback unsuccessful
- Performance unexplained by query patterns or cluster load

Include in escalation:
- Full operator logs: `kubectl logs deployment/clickhouse`
- ClickHouse pod logs from affected replicas
- CRD YAML: `kubectl get ClickhouseInstallation -o yaml`
- ZooKeeper state dump (replication paths)
- `system.replicas` and `system.replication_queue` output
- Metrics from `system.metrics` and `system.events`

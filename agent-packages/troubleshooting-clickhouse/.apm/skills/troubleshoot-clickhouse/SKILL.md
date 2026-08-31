---
name: troubleshoot-clickhouse
description: Diagnose and resolve failures in operator-managed ClickHouse clusters — replicated tables, sharded topology, ZooKeeper coordination, backups, and service components (site-manager, backup-orchestrator, dbaas-adapter, sidecar, secret-monitor, hook). Use whenever the user reports a pod crash or restart loop, a read-only replica, ZooKeeper connection loss after switchover, memory limit exceeded errors, DNS errors, rejected inserts, too many parts or mutations, replication lag, disk usage alerts, long-running queries, too many connections, data loss alerts, or a Helm values configuration question. Match the reported symptom to documented fixes; fall back to a general diagnostic checklist when nothing matches.
---

## How to use the reference file

1. Grep issue headers: `grep -n "^## " references/Troubleshooting.md`
2. If no results: `grep -n "^##[^# ]" references/Troubleshooting.md`
3. Match the user's symptom against the jump table below to select the right section.
4. Load only that section — not the entire file.

## Symptom → reference section

| Symptom | Section in references/Troubleshooting.md |
|---------|------------------------------------------------|
| Query processing error / broken connection | ## Query Processing |
| ZooKeeper connection loss after DR switchover | ## ClickHouse lost connection to Zookeeper after switchover |
| Memory limit for query exceeded / OOMKilled | ## Memory limit (for query) exceeded |
| RENAME EXCHANGE not supported / kernel issue | ## RENAME EXCHANGE is not supported |
| Metrics exporter down | ## ClickHouseMetricsExporterDown |
| ClickHouse pods not running | ## ClickHouseServerDown |
| Metrics exporter fetch errors | ## ClickHouseMetricsExporterFetchErrors |
| Pod restarted recently | ## ClickHouseServerRestartRecently |
| DNS resolution errors | ## ClickHouseDNSErrors |
| Rejected INSERT queries | ## ClickHouseRejectedInsert |
| Too many parts in partition | ## ClickHouseMaxPartCountForPartition |
| Long-running query | ## ClickHouseLongestRunningQuery |
| Replica in read-only state | ## ClickHouseReadonlyReplica |
| Replication lag / absolute delay | ## ClickHouseReplicasMaxAbsoluteDelay |
| Too many client connections | ## ClickHouseTooManyConnections |
| ZooKeeper hardware exceptions | ## ClickHouseZooKeeperHardwareExceptions |
| Disk usage > 90% | ## ClickHouseDiskUsage |
| Replicated data loss | ## ClickHouseReplicatedDataLoss |
| Too many incomplete mutations | ## ClickHouseTooManyMutations |

## Guardrails

Do not recommend:

- Direct XML config edits inside the container (`/etc/clickhouse-server/config.d/`, `/etc/clickhouse-server/users.d/`) — these are overwritten on next reconcile by the operator
- `SYSTEM RESTART REPLICA` without first confirming ZooKeeper connectivity and path consistency
- Manual deletion of ZooKeeper znodes for table replication paths
- Scaling replicas down to zero — at least one replica must remain available
- `kubectl exec` for DDL/DML that bypasses the operator reconciliation loop

## Config value conventions

- Helm values are the source of truth — phrase changes as: "set `<value-path>` in Helm values and redeploy"
- Memory/resource profiles: `dev`, `dev-ha`, `small`, `medium`, `large`, `prod`, `prod-nonha`
- ClickHouse configuration profiles via CR: `configuration.profiles.default/<param>`
- Prometheus alert thresholds: `clickhouseCluster.prometheusRules.<paramName>`
- TLS certificates read from `/tls/ca.crt` at runtime

## Cluster conventions

- CRD: `ClickhouseInstallation` (group: `clickhouse.altinity.com`)
- Status field: `.status.status` = "Completed" when healthy
- Two ClickHouse versions maintained: 243 (24.3) and 258 (25.8)
- Service components: site-manager, backup-orchestrator, dbaas-adapter, sidecar, secret-monitor, hook
- DR: active/standby pairs with switchover capability
- At least one replica per shard must remain available before attempting any data-clearing recovery step

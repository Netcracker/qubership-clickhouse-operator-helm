This chapter describes the troubleshooting scenarios and recommendations for ClickHouse Operator.

# Prometheus Alerts

## ClickHouseMetricsExporterDown

### Description

ClickHouse Metrics Exporter down or not able to collect metrics.

### Possible Causes

### Impact

- Metrics for ClickHouse are missing

### Actions for Investigation

- Check status of the clickhouse-operator pod. All the containers should be running.

### Recommended Actions to Resolve Issue

- Check the status of clickhouse-operator pod.
- Review logs for ClickHouse Metrics Exporter container in clickhouse-operator pod for any errors or issues.

## ClickHouseServerDown

### Description

ClickHouse's pods are possibly not running.

### Possible Causes

- Internal ClickHouse issue.
- ClickHouse's pods are not running due to some reasons.

### Impact

- Metrics for ClickHouse are missing

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseMetricsExporterFetchErrors

### Description

Clickhouse Metrics Exporter are not able to collect some of the metrics.

### Possible Causes

- ClickHouse server pods are not running

### Impact

- Metrics for ClickHouse are missing

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse Metrics Exporter pod.

## ClickHouseServerRestartRecently

### Description

`clickhouse-server` process has been start less than `clickhouseCluster.prometheusRules.clickhouseUptime` seconds ago.

### Possible Causes

- Internal ClickHouse issue.
- ClickHouse's pods are not running due to some reasons.

### Impact

- Downtime of ClickHouse due to restart.

### Actions for Investigation

- Check previous ClickHouse pods log to investigate restart reason.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.

## ClickHouseDNSErrors

### Description

DNS errors occurred in ClickHouse logs.

### Possible Causes

- Internal ClickHouse issue.
- DNS resolving issue.

### Impact

- Possible down-time of ClickHouse.

### Actions for Investigation

- Check DNS settings in `/etc/resolve.conf` and `<remote_servers>` part of `/etc/clickhouse-server/`.
- Check ClickHouse documents:
    * [server-settings-remote-servers](https://clickhouse.tech/docs/en/operations/server-configuration-parameters/settings/#server-settings-remote-servers)
    * [server-settings-disable-internal-dns-cache](https://clickhouse.tech/docs/en/operations/server-configuration-parameters/settings/#server-settings-disable-internal-dns-cache)
    * [query_language-system-drop-dns-cache](https://clickhouse.tech/docs/en/query_language/system/#query_language-system-drop-dns-cache)

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseRejectedInsert

### Description

Rejected INSERT queries occurred.

### Possible Causes

- Internal ClickHouse issue.

### Impact

- Some of the queries are not executed and should be retried.

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.
- Check next official documents:
    * [MergeTreeArchitecture](https://clickhouse.tech/docs/en/development/architecture/#merge-tree)
    * [system.parts_log](https://clickhouse.tech/docs/en/operations/system-tables/#system_tables-part-log)
    * [system.merge_tree_settings](https://clickhouse.tech/docs/en/operations/system-tables/#system-merge_tree_settings)

### Recommended Actions to Resolve Issue

- Decrease insert frequency from the Application.

## ClickHouseMaxPartCountForPartition

### Description

Number of parts exceeds the max part count for partition threshold configured by `clickhouseCluster.prometheusRules.maxPartCountForPartitionThreshold` deploy parameter.

### Possible Causes

- Too many insert queries.

### Impact

- Delayed or rejected inserts.

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseLongestRunningQuery

### Description

Long-running query occurred.

### Possible Causes

- Poor performance of the query.
- Misconfiguration of ClickHouse CPU and Memory limits.

### Impact

- N/A

### Actions for Investigation

- Check the system.processes with long queries [system_tables-processes](https://clickhouse.tech/docs/en/operations/system-tables/#system_tables-processes)
- Check output of next command:
```
kubectl exec -n {{ .Release.Namespace }} pod/$(kubectl get pods -n {{ .Release.Namespace }} | grep $( echo {{ $labels.hostname }} | cut -d '.' -f 1) | cut -d \" \" -f 1) -- clickhouse-client -q \"SELECT * FROM system.processes WHERE elapsed >= 600 FORMAT Vertical\" | less
```

### Recommended Actions to Resolve Issue

- Try to analyze problem query and enhance it.

## ClickHouseReadonlyReplica

### Description

ClickHouse have ReplicatedMergeTree tables that are currently in readonly state.

### Possible Causes

- Re-initialization after ZooKeeper session loss.
- Startup without ZooKeeper configured.

### Impact

- Not possible to execute write queries.

### Actions for Investigation

- Kubenetes Nodes have free enough RAM and Disk via `kubectl top node`
- Status of clickhouse-server pods ```kubectl describe -n {{ .Release.Namespace }} pod/$(kubectl get pods -n {{ $labels.exported_namespace }} | grep $( echo {{ $labels.hostname }} | cut -d '.' -f 1) | cut -d \" \" -f 1)``` `}}
- Connection between clickhouse-server pods and ZooKeeper ```kubectl exec -n {{ $labels.exported_namespace }} pod/$(kubectl get pods -n {{ $labels.exported_namespace }} | grep $( echo {{ $labels.hostname }} | cut -d '.' -f 1) | cut -d \" \" -f 1) -- clickhouse-client -q \"SELECT * FROM system.zookeeper WHERE path='/' FORMAT Vertical\"```
- Connection between clickhouse-server pods via Kubernetes Services ```kubectl exec -n {{ $labels.exported_namespace }} pod/$(kubectl get pods -n {{ $labels.exported_namespace }} | grep $( echo {{ $labels.hostname }} | cut -d '.' -f 1) | cut -d \" \" -f 1) -- clickhouse-client -q \"SELECT host_name, errors_count FROM system.clusters WHERE errors_count > 0 FORMAT PrettyCompactMonoBlock\"```
- Status of PersistentVolumeClaims for pods ```kubectl get pvc -n {{ $labels.exported_namespace }}```

### Recommended Actions to Resolve Issue

- Execute next set of queries in ClickHouse pods:

```bash
  ro_tables=$(clickhouse-client --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="SELECT concat(database, '.', table) as table_name FROM system.replicas WHERE is_readonly and database not in ('system', 'default');")
  for ro_table in $ro_tables; do
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] RO Table: $ro_table"
    count_before=$(clickhouse-client --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="select count(*) from $ro_table;")
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] Number of Rows before restore in table: $ro_table is: $count_before"
    clickhouse-client --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="SYSTEM RESTORE REPLICA $ro_table;"
    count_after=$(clickhouse-client --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="select count(*) from $ro_table;")
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] Number of Rows after restore in table: $ro_table is: $count_after"
  done
```

## ClickHouseReplicasMaxAbsoluteDelay

### Description

Replication lag exceeds Max Absolute Delay threshold, configured by `clickhouseCluster.prometheusRules.replicasMaxAbsoluteDelayThreshold` deploy parameter.

### Possible Causes

- Not enough disk space.
- Connectivity issue between ClickHouse pod and ZooKeeper.

### Impact

- Metrics for ClickHouse are missing

### Actions for Investigation

- Check free disk space disks.
- Check network connection between ClickHouse pod and ZooKeeper.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseTooManyConnections

### Description

Number of Open Client Connections exceeds threshold, configured by `clickhouseCluster.prometheusRules.clickHouseConnectionsThreshold` deploy parameter.

### Possible Causes

- Too many open client connections.

### Impact

- Performance degradation.

### Actions for Investigation

- Check the number of open client connections.

### Recommended Actions to Resolve Issue

- Reduce number of open client connections.
- Increase `max_concurrent_queries` ClickHouse configuration parameter.

## ClickHouseZooKeeperHardwareExceptions

### Description

ZooKeeper Hardware Exceptions occurred.

### Possible Causes

- Connectivity issue between ClickHouse and ZooKeeper pods.

### Impact

- Temporary replication issues.

### Actions for Investigation

- Check connectivity between ClickHouse and ZooKeeper pods. 

### Recommended Actions to Resolve Issue

- N/A

## ClickHouseDiskUsage

### Description

Disk Usage is more than 90 percent.

### Possible Causes

- Not enough space in ClickHouse PVCs.

### Impact

- Possible ClickHouse outage.

### Actions for Investigation

-  Execute next query and analyze which database consumes most of the space:

```sql
  select concat(database, '.', table)                         as table,
         formatReadableSize(sum(bytes))                       as size,
         sum(rows)                                            as rows,
         max(modification_time)                               as latest_modification,
         sum(bytes)                                           as bytes_size,
         any(engine)                                          as engine,
         formatReadableSize(sum(primary_key_bytes_in_memory)) as primary_keys_size
  from system.parts
  where active
  group by database, table
  order by bytes_size desc;
```

### Recommended Actions to Resolve Issue

- Manually increase size of PVCs ClickHouse pods.
- Delete unused data.

## ClickHouseReplicatedDataLoss

### Description

Means, the data part that the server wanted doesn't exist on any replica (even on replicas that are offline right now).

### Possible Causes

- Internal ClickHouse issue.

### Impact

- Data loss.

### Actions for Investigation

- Each concurrent SELECT query use memory in JOINs use CPU for running aggregation function and can read a lot of data from disk when scan parts in partitions and utilize disk I/O.
- Each concurrent INSERT query, allocate around 1MB per each column in an inserted table and utilize disk I/O.

### Recommended Actions to Resolve Issue

- Review logs of ClickHouse pods for any errors or issues.

## ClickHouseTooManyMutations

### Description

Too much incomplete system.mutations.

### Possible Causes

- Something wrong with ALTER TABLE DELETE/UPDATE queries.

### Impact

- Performance degradation.

### Actions for Investigation

- Check mutations errors ```kubectl exec -n {{ $labels.exported_namespace }} pod/$(kubectl get pods -n {{ $labels.exported_namespace }} | grep $( echo {{ $labels.hostname }} | cut -d '.' -f 1) | cut -d \" \" -f 1) -- clickhouse-client -q \"SELECT * FROM system.mutations WHERE is_done=0 FORMAT Vertical\"```
- Read about how to run KILL MUTATION [kill-mutation](https://clickhouse.tech/docs/en/sql-reference/statements/kill/#kill-mutation)

### Recommended Actions to Resolve Issue

- Analyze running queries.
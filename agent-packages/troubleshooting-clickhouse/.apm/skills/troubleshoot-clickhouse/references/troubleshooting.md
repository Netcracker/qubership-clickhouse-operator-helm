This section provides the detailed troubleshooting procedures for ClickHouse cluster installations.
It provides instructions on how to detect and fix generic issues. Ensure you have administrator privileges in the OpenShift project or Kubernetes namespace.

# Common Troubleshooting Scenarios

## Query Processing

### Description

If ClickHouse is not able to process the query, it sends an error description to the client. In the clickhouse-client you get a description of the error in the console.

If you are using the HTTP interface, ClickHouse sends the error description in the response body.

### Alerts

Not applicable.

### Stack trace(s)

```
Code: 47, e.displayText() = DB::Exception: Unknown identifier: a. Note that there are no tables (FROM clause) in your query, context: required_names: 'a' source_tables: table_aliases: private_aliases: column_aliases: public_columns: 'a' masked_columns: array_join_columns: source_columns: , e.what() = DB::Exception
```

### How to solve

You might see a message about a broken connection. In this case, you can repeat the query. If the connection breaks every time you perform the query, check the server logs for errors.

### Recommendations

Not Applicable.

## ClickHouse lost connection to Zookeeper after switchover

### Description

After a DR switchover, replicas can go into read-only mode.

### Alerts

Not Applicable.

### Stack trace(s)

```bash
<Error> prom.samples (ReplicatedMergeTreeRestartingThread): void DB::ReplicatedMergeTreeRestartingThread::run(): Code: 999, e.displayText() = Coordination::Exception: All connection tries failed while connecting to ZooKeeper. nodes: 172.30.161.184:2181
Code: 209, e.displayText() = DB::NetException: Timeout exceeded while reading from socket (172.30.161.184:2181): while receiving handshake from ZooKeeper (version 20.4.5.36 (official build)), 172.30.161.184:2181
```

### How to solve

Try to check Zookeeper connection through client:

1. Open the terminal of Zookeeper pod
2. Try to connect with client:

```bash
bin/zkCli.sh -server 127.0.0.1:2181
```

If connection is unstable with error:

```bash
[WARN] Client session timed out, have not heard from server in 30030ms for sessionid 0x0
[INFO] Client session timed out, have not heard from server in 30030ms for sessionid 0x0, closing socket connection and attempting reconnect
```

It means Zookeeper cluster didn't start and needs to be restarted.

### Recommendations

Restart ZooKeeper cluster.

## Memory limit (for query) exceeded

### Description

This situation could happen if the clients are requesting large amount of data via the query.

### Alerts

Not applicable.

### Stack trace(s)

```
<Error> TCPHandler: Code: 241. DB::Exception: Memory limit (for query) exceeded: would use 9.39 GiB (attempt to allocate chunk of 104185280 bytes), maximum: 9.31 GiB: While executing JoiningTransform. (MEMORY_LIMIT_EXCEEDED)
<Error> auto DB::IBackgroundJobExecutor::execute(DB::JobAndPool)::(anonymous class)::operator()() const: Code: 74. DB::ErrnoException: Cannot read from file, errno: 12, strerror: Cannot allocate memory.
```

### How to solve

1. Complement CR with additional parameters with profiles options:

Use Helm to upgrade parameters:

```yaml
configuration:
  profiles:
    default/max_memory_usage: 12000000000
    default/max_memory_usage_for_all_queries: 160000000000
    default/max_query_size: 524288
    default/use_uncompressed_cache: 1
```

2. Enlarge resources for clickhouse node statefulsets:

```yaml
resources:
  limits:
    cpu: "16"
    memory: 32Gi
```

### Recommendations

Values may be different depending on your own specific case.

## RENAME EXCHANGE is not supported

### Description

The client queries are failing.

### Alerts

Not applicable.

### Stack trace(s)

```
Code: 48. DB::Exception: RENAME EXCHANGE is not supported. (NOT_IMPLEMENTED) (version 22.8.15.23 (official build)))
```

### How to solve

There is an issue in ClickHouse Github repo (https://github.com/ClickHouse/ClickHouse/issues/41024).

The root cause is that some Kernel functions are not implemented for your kernel version.

The fix is to upgrade the kernel version on Kubernetes worker nodes or migrate to a different OS.

### Recommendations

The main prerequisite for ClickHouse is the kernel version is higher than 3.10 for the Kubernetes Worker Nodes.

# Prometheus Alerts Troubleshooting

## ClickHouseMetricsExporterDown

### Description

ClickHouse Metrics Exporter down or not able to collect metrics.

### Possible Causes

- Internal issue with the metrics exporter container.

### Impact

- Metrics for ClickHouse are missing.

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

- Metrics for ClickHouse are missing.

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseMetricsExporterFetchErrors

### Description

ClickHouse Metrics Exporter is not able to collect some of the metrics.

### Possible Causes

- ClickHouse server pods are not running.

### Impact

- Metrics for ClickHouse are missing.

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse Metrics Exporter pod.

## ClickHouseServerRestartRecently

### Description

`clickhouse-server` process has been started less than the configured uptime threshold seconds ago.

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
- Check ClickHouse documents on server-settings-remote-servers, server-settings-disable-internal-dns-cache, and query_language-system-drop-dns-cache.

### Recommended Actions to Resolve Issue

- Try to analyze any errors in ClickHouse pods.
- Restart or redeploy ClickHouse pods if they are in a failed state.

## ClickHouseRejectedInsert

### Description

Rejected INSERT queries occurred.

### Possible Causes

- Internal ClickHouse issue (too many parts for partition).

### Impact

- Some of the queries are not executed and should be retried.

### Actions for Investigation

- Check the status of ClickHouse pods.
- Review logs of ClickHouse pods for any errors or issues.
- Check system.parts_log and system.merge_tree_settings tables.

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
- Decrease insert frequency or batch inserts into larger blocks.

## ClickHouseLongestRunningQuery

### Description

Long-running query occurred.

### Possible Causes

- Poor performance of the query.
- Misconfiguration of ClickHouse CPU and Memory limits.

### Impact

- Resource contention for other queries.

### Actions for Investigation

- Check the system.processes for long queries:
```sql
SELECT * FROM system.processes WHERE elapsed >= 600 FORMAT Vertical
```

### Recommended Actions to Resolve Issue

- Try to analyze the problem query and optimize it.

## ClickHouseReadonlyReplica

### Description

ClickHouse has ReplicatedMergeTree tables that are currently in readonly state.

### Possible Causes

- Re-initialization after ZooKeeper session loss.
- Startup without ZooKeeper configured.

### Impact

- Not possible to execute write queries.

### Actions for Investigation

- Kubernetes Nodes have free enough RAM and Disk via `kubectl top node`
- Status of clickhouse-server pods: `kubectl describe pod <pod-name> -n <namespace>`
- Connection between clickhouse-server pods and ZooKeeper:
```sql
SELECT * FROM system.zookeeper WHERE path='/' FORMAT Vertical
```
- Connection between clickhouse-server pods via Kubernetes Services:
```sql
SELECT host_name, errors_count FROM system.clusters WHERE errors_count > 0 FORMAT PrettyCompactMonoBlock
```
- Status of PersistentVolumeClaims: `kubectl get pvc -n <namespace>`

### Recommended Actions to Resolve Issue

Execute the following to restore read-only replicas:

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

- Stale reads from lagging replicas.

### Actions for Investigation

- Check free disk space.
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

- Verify network policies and pod-to-pod connectivity.

## ClickHouseDiskUsage

### Description

Disk Usage is more than 90 percent.

### Possible Causes

- Not enough space in ClickHouse PVCs.

### Impact

- Possible ClickHouse outage.

### Actions for Investigation

Execute the following query to analyze which database consumes most of the space:

```sql
SELECT concat(database, '.', table) AS table,
       formatReadableSize(sum(bytes)) AS size,
       sum(rows) AS rows,
       max(modification_time) AS latest_modification,
       sum(bytes) AS bytes_size,
       any(engine) AS engine,
       formatReadableSize(sum(primary_key_bytes_in_memory)) AS primary_keys_size
FROM system.parts
WHERE active
GROUP BY database, table
ORDER BY bytes_size DESC;
```

### Recommended Actions to Resolve Issue

- Manually increase size of PVCs for ClickHouse pods.
- Delete unused data.

## ClickHouseReplicatedDataLoss

### Description

The data part that the server wanted doesn't exist on any replica (even on replicas that are offline right now).

### Possible Causes

- Internal ClickHouse issue.

### Impact

- Data loss.

### Actions for Investigation

- Each concurrent SELECT query uses memory in JOINs, uses CPU for running aggregation functions, and can read a lot of data from disk.
- Each concurrent INSERT query allocates around 1MB per each column in an inserted table and utilizes disk I/O.

### Recommended Actions to Resolve Issue

- Review logs of ClickHouse pods for any errors or issues.

## ClickHouseTooManyMutations

### Description

Too many incomplete system.mutations.

### Possible Causes

- Something wrong with ALTER TABLE DELETE/UPDATE queries.

### Impact

- Performance degradation.

### Actions for Investigation

Check mutations errors:
```sql
SELECT * FROM system.mutations WHERE is_done=0 FORMAT Vertical
```

Read about how to run KILL MUTATION: https://clickhouse.tech/docs/en/sql-reference/statements/kill/#kill-mutation

### Recommended Actions to Resolve Issue

- Analyze running mutations and kill stuck ones if needed.

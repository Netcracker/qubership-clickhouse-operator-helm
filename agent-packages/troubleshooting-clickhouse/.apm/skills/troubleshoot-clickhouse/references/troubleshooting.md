[Back](../../README.md)

The following topics are covered in this section:

- [Introduction](#introduction)
- [Common Troubleshooting Scenarios](#common-troubleshooting-scenarios)
    - [Query Processing](#query-processing)
    - [ClickHouse lost connection to Zookeeper after switchover](#clickHouse-lost-connection-to-zookeeper-after-switchover)
    - [Memory limit (for query) exceeded](#memory-limit-(for-query)-exceeded)
    - [RENAME EXCHANGE is not supported](#arangodb-follower-server-pod-logs-contain-an-error-about-stale-or-corrupted-data)

# Introduction

This section provides the detailed troubleshooting procedures for ClickHouse cluster installations.
It provides instructions on how to detect and fix generic issues. Ensure you have administrator privileges in the OpenShift project or Kubernetes namespace.

# Common Troubleshooting Scenarios

This section covers all the known issues that might arise during the ClickHouse Cluster rollout or operation along with the proposed solutions for these issues.

## Query Processing

### Description

If ClickHouse is not able to process the query, it sends an error description to the client. In the clickhouse-client you get a description of the error in the console. 

If you are using the HTTP interface, ClickHouse sends the error description in the response body.

### Alerts

Not applicable.

### Stack trace(s)

You will something similar to:

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
Code: 209, e.displayText() = DB::NetException: Timeout exceeded while reading from socket (172.30.161.184:2181): while receiving handshake from ZooKeeper (version 20.4.5.36 (official build)), 172.30.161.184:2181
Code: 209, e.displayText() = DB::NetException: Timeout exceeded while reading from socket (172.30.161.184:2181): while receiving handshake from ZooKeeper (version 20.4.5.36 (official build)), 172.30.161.184:2181
(Connection loss), Stack trace (when copying this message, always include the lines below):
```

### How to solve

Try to check Zookeeper connection trough client:

1. Open the terminal of Zookeeper pod:
2. Try to connect with client:

```bash
bin/zkCli.sh -server 127.0.0.1:2181
```

If connection will be unstable with error:

```bash
[2020-08-05T16:15:09,653][WARN][zk_id=127.0.0.1:2181][thread=main-SendThread(127.0.0.1:2181)][class=ClientCnxn$SendThread@1190] Client session timed out, have not heard from server in 30030ms for sessionid 0x0
[2020-08-05T16:15:09,654][INFO][zk_id=127.0.0.1:2181][thread=main-SendThread(127.0.0.1:2181)][class=ClientCnxn$SendThread@1238] Client session timed out, have not heard from server in 30030ms for sessionid 0x0, closing socket connection and attempting reconnect
[2020-08-05T16:15:10,953][INFO][zk_id=127.0.0.1:2181][thread=main-SendThread(127.0.0.1:2181)][class=ClientCnxn$SendThread@1112] Opening socket connection to server localhost/127.0.0.1:2181. Will not attempt to authenticate using SASL (unknown error)
[2020-08-05T16:15:10,954][INFO][zk_id=127.0.0.1:2181][thread=main-SendThread(127.0.0.1:2181)][class=ClientCnxn$SendThread@959] Socket connection established, initiating session, client: /127.0.0.1:43630, server: localhost/127.0.0.1:2181
```

It means Zookeeper cluster didn't start and need to restart Zookeeper.

### Recommendations

Restart ZooKeeper cluster.

## Memory limit (for query) exceeded

### Description

This situation could happen if the clients are requestion large amount of data via the query.

### Alerts

Not applicable.

### Stack trace(s)

```
<Error> TCPHandler: Code: 241. DB::Exception: Memory limit (for query) exceeded: would use 9.39 GiB (attempt to allocate chunk of 104185280 bytes), maximum: 9.31 GiB: While executing JoiningTransform. (MEMORY_LIMIT_EXCEEDED)
<Error> auto DB::IBackgroundJobExecutor::execute(DB::JobAndPool)::(anonymous class)::operator()() const: Code: 74. DB::ErrnoException: Cannot read from file /var/lib/clickhouse/data/store/dca/dca688f5-258d-4957-9ca6-88f5258d2957/202206_1322689_1323607_178/ProfileEvent_HedgedRequestsChangeReplica.mrk2, errno: 12, strerror: Cannot allocate memory.
```

### How to solve

1. Complement CR with additional parameters with profiles options described below:

Please use Helm to upgrade parameters refer to [How To Deploy](../../README.md#how-to-deploy)
   
```
  configuration:
    profiles:
      default/max_memory_usage: 12000000000
      default/max_memory_usage_for_all_queries: 160000000000
      default/max_query_size: 524288
      default/use_uncompressed_cache: 1
```
More info you may find here [Configuration parameters](../internal/dev/parameters.md#configuration)

2. Enlarge resource for clickhouse node statefullsets up to:  

Please use Helm to upgrade parameters refer to [How To Deploy](../../README.md#how-to-deploy)

```
  resources:
    limits:
      cpu: "16"
      memory: 32Gi
```

### Recommendations

***Note:*** Values may be different depends on your own specific case

## RENAME EXCHANGE is not supported

### Description

The client queries are failing.

### Alerts

Not applicable

### Stack trace(s)

```
Code: 48. DB::Exception: RENAME EXCHANGE is not supported. (NOT_IMPLEMENTED) (version 22.8.15.23 (official build)))
```

### How to solve

There is an issue in [ClickHouse Github repo](https://github.com/ClickHouse/ClickHouse/issues/41024).

The root cause of this issue is that some of the Kernel functions are not implemented for your kernel version.

The fix is to upgrade the kernel version on Kubernetes worker nodes or migrate to a different OS.

### Recommendations

The main prerequisite for ClickHouse is the kernel version is higher than 3.10 for the Kubernetes Worker Nodes.

# qubership-clickhouse-operator-helm

ClickHouse Operator provides ClickHouse as a service on Kubernetes and OpenShift.

## Repository structure

* `./helm` - directory with HELM chart for ClickHouse components.
* * `./helm/clickhouse` - directory with HELM chart for ClickHouse.
* * `./helm/clickhouse-services` - directory with HELM chart for ClickHouse suplementary services.
* `./hook` - directory with sources of clickhouse secret lock while deplyoment.
* `./secret-monitor` -  directory with sources of clickhouse secret watcher 